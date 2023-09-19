package limiter

import (
	"context"
	"time"

	"ratelimiter/internal/clock"
	"ratelimiter/internal/store"
)

// SlidingWindowLog stores a timestamp for every request in the window. It is
// exact, at the cost of memory proportional to the limit.
type SlidingWindowLog struct {
	limit  int
	window time.Duration
	store  store.Store
	clock  clock.Clock
}

// Allow implements Limiter.
func (s *SlidingWindowLog) Allow(ctx context.Context, key string) (Decision, error) {
	now := s.clock.Now()
	cutoff := now.Add(-s.window)

	return s.store.Do(ctx, key, func(st *store.State) Decision {
		// Log is sorted ascending; drop everything at or before the cutoff.
		i := 0
		for i < len(st.Log) && !st.Log[i].After(cutoff) {
			i++
		}
		if i > 0 {
			st.Log = append(st.Log[:0], st.Log[i:]...)
		}

		if len(st.Log) < s.limit {
			st.Log = append(st.Log, now)
			return Decision{
				Allowed:   true,
				Limit:     s.limit,
				Remaining: clampNonNegative(s.limit - len(st.Log)),
				ResetAt:   st.Log[0].Add(s.window),
			}
		}

		// At the limit: capacity frees when the oldest entry exits the window.
		reset := st.Log[0].Add(s.window)
		return Decision{
			Allowed:    false,
			Limit:      s.limit,
			Remaining:  0,
			RetryAfter: reset.Sub(now),
			ResetAt:    reset,
		}
	})
}
