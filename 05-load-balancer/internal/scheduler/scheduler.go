// Package scheduler decides which backend serves a request.
package scheduler

import (
	"net/http"
	"sync/atomic"

	"github.com/example/lb/internal/backend"
)

// Scheduler picks a backend from the healthy set. Implementations must be safe
// for concurrent use.
type Scheduler interface {
	// Next returns the backend to serve r, or nil if healthy is empty.
	Next(r *http.Request, healthy []*backend.Backend) *backend.Backend
}

// RoundRobin cycles through the healthy backends in order.
type RoundRobin struct {
	counter atomic.Uint64
}

// NewRoundRobin returns a round-robin scheduler.
func NewRoundRobin() *RoundRobin { return &RoundRobin{} }

// Next returns the next backend in rotation. Taking the counter modulo the
// current length keeps this safe when the healthy set shrinks between calls.
func (rr *RoundRobin) Next(_ *http.Request, healthy []*backend.Backend) *backend.Backend {
	n := uint64(len(healthy))
	if n == 0 {
		return nil
	}
	i := rr.counter.Add(1) - 1
	return healthy[i%n]
}

// Name returns the strategy's config identifier.
func (rr *RoundRobin) Name() string { return "round_robin" }
