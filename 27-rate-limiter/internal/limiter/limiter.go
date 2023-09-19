// Package limiter implements token bucket, fixed window, sliding window log
// and sliding window counter rate limiting. Per-key state lives in a
// store.Store (memory or Redis) and time comes from a clock.Clock.
package limiter

import (
	"context"
	"fmt"
	"time"

	"ratelimiter/internal/clock"
	"ratelimiter/internal/store"
)

// Decision is re-exported from the store package so callers depend only on
// limiter.
type Decision = store.Decision

// Limiter decides whether the request identified by key may proceed now.
// Implementations are safe for concurrent use.
type Limiter interface {
	Allow(ctx context.Context, key string) (Decision, error)
}

// Algo names a rate-limiting algorithm.
type Algo string

const (
	TokenBucketAlgo    Algo = "token-bucket"
	FixedWindowAlgo    Algo = "fixed-window"
	SlidingLogAlgo     Algo = "sliding-log"
	SlidingCounterAlgo Algo = "sliding-counter"
)

// Config holds the tunable parameters shared by the algorithms. Not every
// field is meaningful for every algorithm (RefillPerSec is token-bucket only).
type Config struct {
	// Limit is the bucket capacity (token bucket) or the maximum number of
	// requests permitted per window (the other three).
	Limit int
	// Window is the refill reference window or the fixed/sliding window size.
	Window time.Duration
	// RefillPerSec is tokens added per second (token bucket only).
	RefillPerSec float64
}

// New constructs the limiter for algo using the supplied store and clock.
func New(algo Algo, cfg Config, st store.Store, c clock.Clock) (Limiter, error) {
	if cfg.Limit <= 0 {
		return nil, fmt.Errorf("limiter: limit must be > 0, got %d", cfg.Limit)
	}
	switch algo {
	case TokenBucketAlgo:
		refill := cfg.RefillPerSec
		if refill <= 0 {
			return nil, fmt.Errorf("limiter: refill must be > 0 for token bucket, got %v", refill)
		}
		return &TokenBucket{capacity: float64(cfg.Limit), refillPerSec: refill, store: st, clock: c}, nil
	case FixedWindowAlgo:
		if cfg.Window <= 0 {
			return nil, fmt.Errorf("limiter: window must be > 0, got %v", cfg.Window)
		}
		return &FixedWindow{limit: cfg.Limit, window: cfg.Window, store: st, clock: c}, nil
	case SlidingLogAlgo:
		if cfg.Window <= 0 {
			return nil, fmt.Errorf("limiter: window must be > 0, got %v", cfg.Window)
		}
		return &SlidingWindowLog{limit: cfg.Limit, window: cfg.Window, store: st, clock: c}, nil
	case SlidingCounterAlgo:
		if cfg.Window <= 0 {
			return nil, fmt.Errorf("limiter: window must be > 0, got %v", cfg.Window)
		}
		return &SlidingWindowCounter{limit: cfg.Limit, window: cfg.Window, store: st, clock: c}, nil
	default:
		return nil, fmt.Errorf("limiter: unknown algorithm %q", algo)
	}
}

func clampNonNegative(n int) int {
	if n < 0 {
		return 0
	}
	return n
}
