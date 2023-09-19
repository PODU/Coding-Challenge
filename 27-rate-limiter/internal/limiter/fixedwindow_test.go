package limiter

import (
	"testing"
	"time"

	"ratelimiter/internal/clock"
	"ratelimiter/internal/store"
)

func newLimiter(t *testing.T, algo Algo, cfg Config, clk clock.Clock) Limiter {
	t.Helper()
	st := store.NewMemoryStore(clk, 0)
	t.Cleanup(func() { _ = st.Close() })
	l, err := New(algo, cfg, st, clk)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return l
}

func TestFixedWindow_AllowUpToThresholdThenReject(t *testing.T) {
	// Start aligned to a 60s window boundary.
	clk := clock.NewFakeClock(time.Unix(60, 0))
	l := newLimiter(t, FixedWindowAlgo, Config{Limit: 60, Window: 60 * time.Second}, clk)

	for i := 0; i < 60; i++ {
		if !mustAllow(t, l, "ip").Allowed {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if mustAllow(t, l, "ip").Allowed {
		t.Fatal("61st request in window should reject")
	}
}

func TestFixedWindow_ResetsOnRoll(t *testing.T) {
	clk := clock.NewFakeClock(time.Unix(60, 0))
	l := newLimiter(t, FixedWindowAlgo, Config{Limit: 5, Window: 60 * time.Second}, clk)

	for i := 0; i < 5; i++ {
		mustAllow(t, l, "ip")
	}
	if mustAllow(t, l, "ip").Allowed {
		t.Fatal("over limit in window 1")
	}
	clk.Advance(60 * time.Second) // roll to next window.
	if !mustAllow(t, l, "ip").Allowed {
		t.Fatal("counter should reset in window 2")
	}
}

// TestFixedWindow_BoundaryDoubling demonstrates the known weakness: a burst at
// the very end of window N plus a burst at the start of N+1 admits up to 2x the
// limit within a span shorter than one window.
func TestFixedWindow_BoundaryDoubling(t *testing.T) {
	limit := 5
	window := 60 * time.Second
	// 1 second before a window boundary.
	clk := clock.NewFakeClock(time.Unix(120, 0).Add(-time.Second))
	l := newLimiter(t, FixedWindowAlgo, Config{Limit: limit, Window: window}, clk)

	allowed := 0
	for i := 0; i < limit; i++ { // fill window N at its tail.
		if mustAllow(t, l, "ip").Allowed {
			allowed++
		}
	}
	clk.Advance(2 * time.Second) // cross the boundary into window N+1.
	for i := 0; i < limit; i++ { // fill window N+1 at its head.
		if mustAllow(t, l, "ip").Allowed {
			allowed++
		}
	}
	if allowed != 2*limit {
		t.Fatalf("expected boundary doubling to admit %d within ~2s, got %d", 2*limit, allowed)
	}
}
