package limiter

import (
	"testing"
	"time"

	"ratelimiter/internal/clock"
)

func TestSlidingWindowLog_PreciseLimit(t *testing.T) {
	clk := clock.NewFakeClock(time.Unix(1000, 0))
	l := newLimiter(t, SlidingLogAlgo, Config{Limit: 3, Window: 10 * time.Second}, clk)

	for i := 0; i < 3; i++ {
		if !mustAllow(t, l, "ip").Allowed {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if mustAllow(t, l, "ip").Allowed {
		t.Fatal("4th request within window should reject")
	}
}

func TestSlidingWindowLog_PrunesOldTimestamps(t *testing.T) {
	clk := clock.NewFakeClock(time.Unix(1000, 0))
	l := newLimiter(t, SlidingLogAlgo, Config{Limit: 3, Window: 10 * time.Second}, clk)

	for i := 0; i < 3; i++ {
		mustAllow(t, l, "ip")
	}
	// Advance past the window: all three timestamps fall out, full allowance.
	clk.Advance(11 * time.Second)
	for i := 0; i < 3; i++ {
		if !mustAllow(t, l, "ip").Allowed {
			t.Fatalf("after prune, request %d should be allowed", i+1)
		}
	}
	if mustAllow(t, l, "ip").Allowed {
		t.Fatal("4th after prune should reject again")
	}
}

// TestSlidingWindowLog_NoBoundaryDoubling contrasts with the fixed window:
// the same end-of-window + start-of-next burst pattern does NOT admit 2x,
// because the sliding window always looks back exactly one window.
func TestSlidingWindowLog_NoBoundaryDoubling(t *testing.T) {
	limit := 5
	window := 60 * time.Second
	clk := clock.NewFakeClock(time.Unix(1000, 0))
	l := newLimiter(t, SlidingLogAlgo, Config{Limit: limit, Window: window}, clk)

	allowed := 0
	for i := 0; i < limit; i++ {
		if mustAllow(t, l, "ip").Allowed {
			allowed++
		}
	}
	clk.Advance(2 * time.Second) // small step, still well within the window.
	for i := 0; i < limit; i++ {
		if mustAllow(t, l, "ip").Allowed {
			allowed++
		}
	}
	if allowed != limit {
		t.Fatalf("sliding log must cap at %d across a 2s span, got %d", limit, allowed)
	}
}

func TestSlidingWindowLog_PartialPrune(t *testing.T) {
	clk := clock.NewFakeClock(time.Unix(1000, 0))
	l := newLimiter(t, SlidingLogAlgo, Config{Limit: 2, Window: 10 * time.Second}, clk)

	mustAllow(t, l, "ip") // t=1000
	clk.Advance(6 * time.Second)
	mustAllow(t, l, "ip") // t=1006, now 2 in window -> full
	if mustAllow(t, l, "ip").Allowed {
		t.Fatal("at limit, should reject")
	}
	// Advance so only the first (t=1000) expires: t=1011, window=[1001,1011].
	clk.Advance(5 * time.Second) // t=1011
	if !mustAllow(t, l, "ip").Allowed {
		t.Fatal("one slot should have freed after partial prune")
	}
}
