package limiter

import (
	"testing"
	"time"

	"ratelimiter/internal/clock"
)

// fillWindow sends n requests and returns how many were allowed.
func countAllowed(t *testing.T, l Limiter, key string, n int) int {
	t.Helper()
	allowed := 0
	for i := 0; i < n; i++ {
		if mustAllow(t, l, key).Allowed {
			allowed++
		}
	}
	return allowed
}

func TestSlidingWindowCounter_FreshWindow(t *testing.T) {
	clk := clock.NewFakeClock(time.Unix(0, 0))
	l := newLimiter(t, SlidingCounterAlgo, Config{Limit: 10, Window: 100 * time.Second}, clk)
	// Cold key, prev=0: behaves like a plain counter, exactly limit allowed.
	if got := countAllowed(t, l, "ip", 15); got != 10 {
		t.Fatalf("fresh window: expected 10 allowed, got %d", got)
	}
}

// At 40% into a new window with a full previous window, the previous window is
// weighted by 60%: estimated = curr + 10*0.6 = curr + 6, so only 4 more pass.
func TestSlidingWindowCounter_WeightedAt40Percent(t *testing.T) {
	clk := clock.NewFakeClock(time.Unix(0, 0))
	l := newLimiter(t, SlidingCounterAlgo, Config{Limit: 10, Window: 100 * time.Second}, clk)

	countAllowed(t, l, "ip", 15) // saturate window 1 -> prevCount becomes 10.
	clk.Set(time.Unix(140, 0))   // 40% into window 2.

	if got := countAllowed(t, l, "ip", 15); got != 4 {
		t.Fatalf("at 40%%: expected 4 allowed (10*0.6 weight), got %d", got)
	}
}

// At the very start of a new window the full previous window is weighted 100%,
// so the estimate already equals the limit and nothing passes.
func TestSlidingWindowCounter_WeightedAtBoundary(t *testing.T) {
	clk := clock.NewFakeClock(time.Unix(0, 0))
	l := newLimiter(t, SlidingCounterAlgo, Config{Limit: 10, Window: 100 * time.Second}, clk)

	countAllowed(t, l, "ip", 15)
	clk.Set(time.Unix(100, 0)) // 0% into window 2, prev weight = 1.0.

	if got := countAllowed(t, l, "ip", 5); got != 0 {
		t.Fatalf("at boundary with full prev window: expected 0 allowed, got %d", got)
	}
}

// Near the end of the window the previous-window contribution has decayed to
// almost zero, so nearly the full limit is available again.
func TestSlidingWindowCounter_DecaysAt99Percent(t *testing.T) {
	clk := clock.NewFakeClock(time.Unix(0, 0))
	l := newLimiter(t, SlidingCounterAlgo, Config{Limit: 10, Window: 100 * time.Second}, clk)

	countAllowed(t, l, "ip", 15)
	clk.Set(time.Unix(199, 0)) // 99% into window 2, prev weight = 0.01.

	if got := countAllowed(t, l, "ip", 15); got != 10 {
		t.Fatalf("at 99%%: expected 10 allowed (prev decayed), got %d", got)
	}
}

// Skipping more than one full window clears the previous-window contribution.
func TestSlidingWindowCounter_GapClearsPrevious(t *testing.T) {
	clk := clock.NewFakeClock(time.Unix(0, 0))
	l := newLimiter(t, SlidingCounterAlgo, Config{Limit: 10, Window: 100 * time.Second}, clk)

	countAllowed(t, l, "ip", 15)
	clk.Set(time.Unix(250, 0)) // two windows later: previous must reset to 0.

	if got := countAllowed(t, l, "ip", 15); got != 10 {
		t.Fatalf("after multi-window gap: expected 10 allowed, got %d", got)
	}
}
