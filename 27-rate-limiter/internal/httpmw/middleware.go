// Package httpmw is HTTP middleware that applies a limiter.Limiter.
package httpmw

import (
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ratelimiter/internal/limiter"
)

// Config configures the RateLimit middleware.
type Config struct {
	// KeyFn derives the client key from a request. If nil, ClientIPKey("") is
	// used (RemoteAddr only, no trusted proxy header).
	KeyFn func(*http.Request) string
	// FailOpen lets requests through when the limiter errors; otherwise they
	// get a 503.
	FailOpen bool
	// Logger receives backend errors; if nil, log.Default() is used.
	Logger *log.Logger
}

// ClientIPKey keys requests by client IP. If trustedHeader is set and present,
// its first value is used, which is only safe behind a proxy you control.
// Otherwise the host part of RemoteAddr is used.
func ClientIPKey(trustedHeader string) func(*http.Request) string {
	return func(r *http.Request) string {
		if trustedHeader != "" {
			if v := r.Header.Get(trustedHeader); v != "" {
				// X-Forwarded-For may be a comma-separated list; take the first.
				if i := strings.IndexByte(v, ','); i >= 0 {
					v = v[:i]
				}
				return strings.TrimSpace(v)
			}
		}
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			return r.RemoteAddr
		}
		return host
	}
}

// RateLimit wraps the next handler, consulting l for each request.
func RateLimit(l limiter.Limiter, cfg Config) func(http.Handler) http.Handler {
	keyFn := cfg.KeyFn
	if keyFn == nil {
		keyFn = ClientIPKey("")
	}
	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFn(r)
			dec, err := l.Allow(r.Context(), key)
			if err != nil {
				logger.Printf("ratelimit: backend error for key %q: %v", key, err)
				if cfg.FailOpen {
					next.ServeHTTP(w, r)
					return
				}
				http.Error(w, "Rate Limiter Unavailable", http.StatusServiceUnavailable)
				return
			}

			setRateLimitHeaders(w, dec)
			if dec.Allowed {
				next.ServeHTTP(w, r)
				return
			}

			retry := int(secondsCeil(dec.RetryAfter))
			if retry > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(retry))
			}
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte("Too Many Requests"))
		})
	}
}

func setRateLimitHeaders(w http.ResponseWriter, dec limiter.Decision) {
	h := w.Header()
	h.Set("X-RateLimit-Limit", strconv.Itoa(dec.Limit))
	h.Set("X-RateLimit-Remaining", strconv.Itoa(dec.Remaining))
	if !dec.ResetAt.IsZero() {
		h.Set("X-RateLimit-Reset", strconv.FormatInt(dec.ResetAt.Unix(), 10))
	}
}

// secondsCeil rounds up, so Retry-After never tells a client to come back too
// early.
func secondsCeil(d time.Duration) int64 {
	if d <= 0 {
		return 0
	}
	return int64((d + time.Second - 1) / time.Second)
}
