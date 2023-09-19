package limiter

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ratelimiter/internal/clock"
	"ratelimiter/internal/store"
)

// TestConcurrency_NeverExceedsLimit fires many goroutines at a single key with
// a frozen clock (no refill) and asserts the total number admitted equals the
// limit exactly, for every algorithm. Run with -race to catch data races in
// the read-modify-write path.
func TestConcurrency_NeverExceedsLimit(t *testing.T) {
	cases := []struct {
		name string
		algo Algo
		cfg  Config
	}{
		{"token-bucket", TokenBucketAlgo, Config{Limit: 50, RefillPerSec: 1}},
		{"fixed-window", FixedWindowAlgo, Config{Limit: 50, Window: time.Hour}},
		{"sliding-log", SlidingLogAlgo, Config{Limit: 50, Window: time.Hour}},
		{"sliding-counter", SlidingCounterAlgo, Config{Limit: 50, Window: time.Hour}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clk := clock.NewFakeClock(time.Unix(0, 0))
			st := store.NewMemoryStore(clk, 0)
			t.Cleanup(func() { _ = st.Close() })
			l, err := New(tc.algo, tc.cfg, st, clk)
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			const goroutines = 200
			var allowed int64
			var wg sync.WaitGroup
			start := make(chan struct{})
			wg.Add(goroutines)
			for i := 0; i < goroutines; i++ {
				go func() {
					defer wg.Done()
					<-start
					dec, err := l.Allow(context.Background(), "shared")
					if err != nil {
						t.Errorf("Allow: %v", err)
						return
					}
					if dec.Allowed {
						atomic.AddInt64(&allowed, 1)
					}
				}()
			}
			close(start)
			wg.Wait()

			if allowed != int64(tc.cfg.Limit) {
				t.Fatalf("%s: expected exactly %d allowed under concurrency, got %d",
					tc.name, tc.cfg.Limit, allowed)
			}
		})
	}
}
