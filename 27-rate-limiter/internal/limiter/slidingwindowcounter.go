package limiter

import (
	"context"
	"math"
	"time"

	"ratelimiter/internal/clock"
	"ratelimiter/internal/store"
)

// SlidingWindowCounter keeps counts for the current and previous fixed windows
// and estimates the rate as
//
//	estimated = currCount + prevCount * (1 - elapsedFractionOfCurrentWindow)
//
// That avoids most of the fixed window's boundary burst while storing only two
// numbers per key.
type SlidingWindowCounter struct {
	limit  int
	window time.Duration
	store  store.Store
	clock  clock.Clock
}

// Allow implements Limiter.
func (s *SlidingWindowCounter) Allow(ctx context.Context, key string) (Decision, error) {
	now := s.clock.Now()

	return s.store.Do(ctx, key, func(st *store.State) Decision {
		curStart := now.Truncate(s.window)

		switch {
		case st.CurrWindowStart.IsZero():
			st.CurrWindowStart = curStart
			st.CurrCount = 0
			st.PrevCount = 0
		case !st.CurrWindowStart.Equal(curStart):
			if curStart.Sub(st.CurrWindowStart) == s.window {
				st.PrevCount = st.CurrCount
			} else {
				// More than one window passed, so the previous one was empty.
				st.PrevCount = 0
			}
			st.CurrCount = 0
			st.CurrWindowStart = curStart
		}

		elapsed := now.Sub(curStart)
		frac := float64(elapsed) / float64(s.window)
		estimated := float64(st.CurrCount) + float64(st.PrevCount)*(1-frac)
		reset := curStart.Add(s.window)

		if estimated < float64(s.limit) {
			st.CurrCount++
			remaining := s.limit - int(math.Ceil(estimated+1))
			return Decision{
				Allowed:   true,
				Limit:     s.limit,
				Remaining: clampNonNegative(remaining),
				ResetAt:   reset,
			}
		}
		return Decision{
			Allowed:    false,
			Limit:      s.limit,
			Remaining:  0,
			RetryAfter: reset.Sub(now),
			ResetAt:    reset,
		}
	})
}
