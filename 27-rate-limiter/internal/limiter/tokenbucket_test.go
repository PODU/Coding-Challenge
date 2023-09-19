package limiter

import (
	"context"
	"testing"
	"time"

	"ratelimiter/internal/clock"
	"ratelimiter/internal/store"
)

func newTokenBucket(t *testing.T, capacity int, refill float64, clk clock.Clock) Limiter {
	t.Helper()
	st := store.NewMemoryStore(clk, 0)
	t.Cleanup(func() { _ = st.Close() })
	l, err := New(TokenBucketAlgo, Config{Limit: capacity, RefillPerSec: refill}, st, clk)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return l
}

func mustAllow(t *testing.T, l Limiter, key string) Decision {
	t.Helper()
	dec, err := l.Allow(context.Background(), key)
	if err != nil {
		t.Fatalf("Allow: %v", err)
	}
	return dec
}

func TestTokenBucket_BurstThenReject(t *testing.T) {
	clk := clock.NewFakeClock(time.Unix(0, 0))
	l := newTokenBucket(t, 10, 1, clk)

	// Drain the full capacity: 10 immediate requests all allowed.
	for i := 0; i < 10; i++ {
		if dec := mustAllow(t, l, "ip"); !dec.Allowed {
			t.Fatalf("request %d: expected allowed", i+1)
		}
	}
	// 11th request with no refill must be rejected.
	dec := mustAllow(t, l, "ip")
	if dec.Allowed {
		t.Fatal("11th request: expected reject")
	}
	if dec.RetryAfter <= 0 {
		t.Fatalf("expected positive RetryAfter, got %v", dec.RetryAfter)
	}
}

func TestTokenBucket_RefillGrantsExactlyOne(t *testing.T) {
	clk := clock.NewFakeClock(time.Unix(0, 0))
	l := newTokenBucket(t, 10, 1, clk)
	for i := 0; i < 10; i++ {
		mustAllow(t, l, "ip")
	}
	// Advance 1s at 1 token/s => exactly one token.
	clk.Advance(time.Second)
	if dec := mustAllow(t, l, "ip"); !dec.Allowed {
		t.Fatal("expected one token after 1s")
	}
	if dec := mustAllow(t, l, "ip"); dec.Allowed {
		t.Fatal("expected only one token after 1s, second should reject")
	}
}

func TestTokenBucket_RefillCappedAtCapacity(t *testing.T) {
	clk := clock.NewFakeClock(time.Unix(0, 0))
	l := newTokenBucket(t, 10, 1, clk)
	for i := 0; i < 10; i++ {
		mustAllow(t, l, "ip")
	}
	// Idle for an hour: refill must cap at capacity (10), not 3600.
	clk.Advance(time.Hour)
	allowed := 0
	for i := 0; i < 20; i++ {
		if mustAllow(t, l, "ip").Allowed {
			allowed++
		}
	}
	if allowed != 10 {
		t.Fatalf("expected refill capped at capacity 10, got %d allowed", allowed)
	}
}

func TestTokenBucket_FractionalRefillRate(t *testing.T) {
	clk := clock.NewFakeClock(time.Unix(0, 0))
	// 0.5 tokens/sec, capacity 1.
	l := newTokenBucket(t, 1, 0.5, clk)
	if !mustAllow(t, l, "ip").Allowed {
		t.Fatal("first request should be allowed (cold start full)")
	}
	if mustAllow(t, l, "ip").Allowed {
		t.Fatal("second immediate request should reject")
	}
	clk.Advance(time.Second) // 0.5 tokens accrued, still < 1.
	if mustAllow(t, l, "ip").Allowed {
		t.Fatal("0.5 tokens should not allow")
	}
	clk.Advance(time.Second) // now 1.0 token.
	if !mustAllow(t, l, "ip").Allowed {
		t.Fatal("1.0 token should allow")
	}
}

func TestTokenBucket_BackwardClockNoFreeTokens(t *testing.T) {
	clk := clock.NewFakeClock(time.Unix(1000, 0))
	l := newTokenBucket(t, 5, 1, clk)
	for i := 0; i < 5; i++ {
		mustAllow(t, l, "ip")
	}
	// Move the clock backward; must not grant tokens.
	clk.Set(time.Unix(0, 0))
	if mustAllow(t, l, "ip").Allowed {
		t.Fatal("backward clock must not refill tokens")
	}
}
