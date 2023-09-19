package limiter

import (
	"context"
	"time"

	"ratelimiter/internal/clock"
	"ratelimiter/internal/store"
)

// FixedWindow counts requests per key in fixed, clock-aligned windows. It is
// cheap, but a burst across a window boundary can get up to twice the limit
// through in a short span.
type FixedWindow struct {
	limit  int
	window time.Duration
	store  store.Store
	clock  clock.Clock
}

// Allow implements Limiter.
func (f *FixedWindow) Allow(ctx context.Context, key string) (Decision, error) {
	now := f.clock.Now()

	return f.store.Do(ctx, key, func(st *store.State) Decision {
		windowStart := now.Truncate(f.window)
		if st.WindowStart.IsZero() || !st.WindowStart.Equal(windowStart) {
			st.WindowStart = windowStart
			st.Count = 0
		}
		reset := windowStart.Add(f.window)

		if st.Count < f.limit {
			st.Count++
			return Decision{
				Allowed:   true,
				Limit:     f.limit,
				Remaining: clampNonNegative(f.limit - st.Count),
				ResetAt:   reset,
			}
		}
		return Decision{
			Allowed:    false,
			Limit:      f.limit,
			Remaining:  0,
			RetryAfter: reset.Sub(now),
			ResetAt:    reset,
		}
	})
}
