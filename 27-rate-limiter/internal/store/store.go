// Package store holds per-key limiter state, in memory or in Redis.
package store

import (
	"context"
	"errors"
	"time"
)

// Decision is the result a limiter returns for one request.
type Decision struct {
	Allowed    bool          // whether the request may proceed now
	Limit      int           // ceiling, surfaced as X-RateLimit-Limit
	Remaining  int           // best-effort remaining allowance (never negative)
	RetryAfter time.Duration // hint for Retry-After when !Allowed
	ResetAt    time.Time     // when the window/bucket next frees capacity
}

// State has the fields of all four algorithms; each uses only its own. It is
// stored as JSON in Redis.
type State struct {
	// Token bucket.
	Tokens     float64   `json:"tokens,omitempty"`
	LastRefill time.Time `json:"last_refill,omitempty"`

	// Fixed window.
	WindowStart time.Time `json:"window_start,omitempty"`
	Count       int       `json:"count,omitempty"`

	// Sliding window counter.
	PrevCount       int       `json:"prev_count,omitempty"`
	CurrCount       int       `json:"curr_count,omitempty"`
	CurrWindowStart time.Time `json:"curr_window_start,omitempty"`

	// Sliding window log (timestamps in ascending order).
	Log []time.Time `json:"log,omitempty"`
}

// Store runs one algorithm step for a key atomically. Implementations must be
// safe for concurrent use.
type Store interface {
	// Do loads key's state (zero if new), runs fn on it, saves any changes
	// and returns fn's Decision, all as one atomic step.
	Do(ctx context.Context, key string, fn func(state *State) Decision) (Decision, error)
	// Close releases any background resources (e.g. the janitor goroutine).
	Close() error
}

// ErrContention is returned when an optimistic Redis transaction could not
// commit after the configured number of retries.
var ErrContention = errors.New("store: too much contention, transaction retries exhausted")
