// Package pool holds the configured backends and hands out snapshots of the
// healthy ones.
package pool

import (
	"sync"

	"github.com/example/lb/internal/backend"
)

// BackendPool holds every configured backend.
type BackendPool struct {
	mu       sync.RWMutex
	backends []*backend.Backend
}

// New creates a pool over backends.
func New(backends []*backend.Backend) *BackendPool {
	return &BackendPool{backends: backends}
}

// All returns a copy of the backend list, healthy or not.
func (p *BackendPool) All() []*backend.Backend {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]*backend.Backend, len(p.backends))
	copy(out, p.backends)
	return out
}

// Healthy returns a new slice of the backends that are currently alive.
func (p *BackendPool) Healthy() []*backend.Backend {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]*backend.Backend, 0, len(p.backends))
	for _, b := range p.backends {
		if b.Alive() {
			out = append(out, b)
		}
	}
	return out
}

// Len returns the number of configured backends.
func (p *BackendPool) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.backends)
}
