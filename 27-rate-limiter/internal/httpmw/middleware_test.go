package httpmw

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ratelimiter/internal/clock"
	"ratelimiter/internal/limiter"
	"ratelimiter/internal/store"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
}

func newFixedWindowLimiter(t *testing.T, limit int, clk clock.Clock) limiter.Limiter {
	t.Helper()
	st := store.NewMemoryStore(clk, 0)
	t.Cleanup(func() { _ = st.Close() })
	l, err := limiter.New(limiter.FixedWindowAlgo, limiter.Config{Limit: limit, Window: time.Minute}, st, clk)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return l
}

func doGet(h http.Handler, remoteAddr string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/limited", nil)
	req.RemoteAddr = remoteAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestMiddleware_AllowsThenRejectsWithHeaders(t *testing.T) {
	clk := clock.NewFakeClock(time.Unix(60, 0))
	l := newFixedWindowLimiter(t, 3, clk)
	h := RateLimit(l, Config{KeyFn: ClientIPKey("")})(okHandler())

	for i := 0; i < 3; i++ {
		rec := doGet(h, "1.2.3.4:5555", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, rec.Code)
		}
		if got := rec.Header().Get("X-RateLimit-Limit"); got != "3" {
			t.Fatalf("expected X-RateLimit-Limit=3, got %q", got)
		}
	}

	rec := doGet(h, "1.2.3.4:5555", nil)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got == "" {
		t.Fatal("expected Retry-After header on 429")
	}
	if got := rec.Header().Get("X-RateLimit-Remaining"); got != "0" {
		t.Fatalf("expected X-RateLimit-Remaining=0, got %q", got)
	}
	if got := rec.Header().Get("X-RateLimit-Reset"); got == "" {
		t.Fatal("expected X-RateLimit-Reset header")
	}
	if body := rec.Body.String(); body != "Too Many Requests" {
		t.Fatalf("unexpected 429 body: %q", body)
	}
}

func TestMiddleware_PerIPIsolation(t *testing.T) {
	clk := clock.NewFakeClock(time.Unix(60, 0))
	l := newFixedWindowLimiter(t, 1, clk)
	h := RateLimit(l, Config{KeyFn: ClientIPKey("")})(okHandler())

	if rec := doGet(h, "1.1.1.1:1", nil); rec.Code != http.StatusOK {
		t.Fatalf("IP1 first: expected 200, got %d", rec.Code)
	}
	if rec := doGet(h, "1.1.1.1:1", nil); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("IP1 second: expected 429, got %d", rec.Code)
	}
	// Different IP has its own bucket.
	if rec := doGet(h, "2.2.2.2:1", nil); rec.Code != http.StatusOK {
		t.Fatalf("IP2 first: expected 200, got %d", rec.Code)
	}
}

func TestMiddleware_TrustedHeaderUsedWhenConfigured(t *testing.T) {
	clk := clock.NewFakeClock(time.Unix(60, 0))
	l := newFixedWindowLimiter(t, 1, clk)
	h := RateLimit(l, Config{KeyFn: ClientIPKey("X-Forwarded-For")})(okHandler())

	// Same RemoteAddr but different forwarded IPs => independent buckets.
	if rec := doGet(h, "10.0.0.1:9", map[string]string{"X-Forwarded-For": "9.9.9.9"}); rec.Code != http.StatusOK {
		t.Fatalf("client A first: expected 200, got %d", rec.Code)
	}
	if rec := doGet(h, "10.0.0.1:9", map[string]string{"X-Forwarded-For": "9.9.9.9"}); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("client A second: expected 429, got %d", rec.Code)
	}
	if rec := doGet(h, "10.0.0.1:9", map[string]string{"X-Forwarded-For": "8.8.8.8"}); rec.Code != http.StatusOK {
		t.Fatalf("client B (different XFF) first: expected 200, got %d", rec.Code)
	}
}

func TestMiddleware_SpoofedHeaderIgnoredWhenNotTrusted(t *testing.T) {
	clk := clock.NewFakeClock(time.Unix(60, 0))
	l := newFixedWindowLimiter(t, 1, clk)
	// No trusted header configured => key is RemoteAddr; spoofed XFF ignored.
	h := RateLimit(l, Config{KeyFn: ClientIPKey("")})(okHandler())

	if rec := doGet(h, "5.5.5.5:1", map[string]string{"X-Forwarded-For": "1.1.1.1"}); rec.Code != http.StatusOK {
		t.Fatalf("first: expected 200, got %d", rec.Code)
	}
	// Attacker rotates the spoofed header but RemoteAddr is unchanged => limited.
	if rec := doGet(h, "5.5.5.5:1", map[string]string{"X-Forwarded-For": "2.2.2.2"}); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second (spoofed header rotated): expected 429, got %d", rec.Code)
	}
}

// failingLimiter always returns an error to exercise fail-open / fail-closed.
type failingLimiter struct{}

func (failingLimiter) Allow(context.Context, string) (limiter.Decision, error) {
	return limiter.Decision{}, errors.New("backend down")
}

func TestMiddleware_FailOpen(t *testing.T) {
	h := RateLimit(failingLimiter{}, Config{KeyFn: ClientIPKey(""), FailOpen: true})(okHandler())
	if rec := doGet(h, "1.2.3.4:1", nil); rec.Code != http.StatusOK {
		t.Fatalf("fail-open: expected 200, got %d", rec.Code)
	}
}

func TestMiddleware_FailClosed(t *testing.T) {
	h := RateLimit(failingLimiter{}, Config{KeyFn: ClientIPKey(""), FailOpen: false})(okHandler())
	if rec := doGet(h, "1.2.3.4:1", nil); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("fail-closed: expected 503, got %d", rec.Code)
	}
}
