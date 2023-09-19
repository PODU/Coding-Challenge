package limiter

import (
	"context"
	"math"
	"time"

	"ratelimiter/internal/clock"
	"ratelimiter/internal/store"
)

// TokenBucket refills each key's bucket at a fixed rate up to its capacity, and
// each request spends one token. Refill is worked out from elapsed time on each
// call rather than by a background goroutine. Tokens are float64 so rates below
// one per second work.
type TokenBucket struct {
	capacity     float64
	refillPerSec float64
	store        store.Store
	clock        clock.Clock
}

// Allow implements Limiter.
func (b *TokenBucket) Allow(ctx context.Context, key string) (Decision, error) {
	now := b.clock.Now()
	limit := int(b.capacity)

	return b.store.Do(ctx, key, func(st *store.State) Decision {
		if st.LastRefill.IsZero() {
			// A new key starts with a full bucket.
			st.Tokens = b.capacity
		} else {
			elapsed := now.Sub(st.LastRefill).Seconds()
			if elapsed > 0 { // a clock that jumps backwards must not add tokens
				st.Tokens = math.Min(b.capacity, st.Tokens+elapsed*b.refillPerSec)
			}
		}
		st.LastRefill = now

		if st.Tokens >= 1 {
			st.Tokens--
			toFull := time.Duration((b.capacity - st.Tokens) / b.refillPerSec * float64(time.Second))
			return Decision{
				Allowed:   true,
				Limit:     limit,
				Remaining: int(st.Tokens),
				ResetAt:   now.Add(toFull),
			}
		}

		// Not enough for one token: wait until one accrues.
		need := 1 - st.Tokens
		retry := time.Duration(need / b.refillPerSec * float64(time.Second))
		return Decision{
			Allowed:    false,
			Limit:      limit,
			Remaining:  0,
			RetryAfter: retry,
			ResetAt:    now.Add(retry),
		}
	})
}
