package store

import (
	"context"
	"sync"
	"time"

	"ratelimiter/internal/clock"
)

type entry struct {
	state      State
	lastAccess time.Time
}

// MemoryStore is a Store for a single process. A janitor evicts idle keys so
// memory doesn't grow with every client ever seen.
type MemoryStore struct {
	mu      sync.Mutex
	entries map[string]*entry
	clock   clock.Clock
	idleTTL time.Duration

	stop chan struct{}
	done chan struct{}
}

// NewMemoryStore returns a MemoryStore. idleTTL is how long a key may sit
// untouched before the janitor may evict it; pass 0 to disable eviction.
func NewMemoryStore(c clock.Clock, idleTTL time.Duration) *MemoryStore {
	return &MemoryStore{
		entries: make(map[string]*entry),
		clock:   c,
		idleTTL: idleTTL,
	}
}

// Do runs fn against key's state under the store mutex, making the
// read-modify-write atomic.
func (s *MemoryStore) Do(_ context.Context, key string, fn func(*State) Decision) (Decision, error) {
	now := s.clock.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.entries[key]
	if !ok {
		e = &entry{}
		s.entries[key] = e
	}
	e.lastAccess = now
	dec := fn(&e.state)
	return dec, nil
}

// Cleanup evicts keys idle for longer than idleTTL, if idleTTL is non-zero.
func (s *MemoryStore) Cleanup() int {
	if s.idleTTL <= 0 {
		return 0
	}
	now := s.clock.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	evicted := 0
	for k, e := range s.entries {
		if now.Sub(e.lastAccess) >= s.idleTTL {
			delete(s.entries, k)
			evicted++
		}
	}
	return evicted
}

// Len reports the number of tracked keys (test helper).
func (s *MemoryStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

// StartJanitor calls Cleanup every interval until Close. The ticker runs on
// real time, but what counts as idle is still decided by the store's clock.
func (s *MemoryStore) StartJanitor(interval time.Duration) {
	if interval <= 0 || s.idleTTL <= 0 {
		return
	}
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	go func() {
		defer close(s.done)
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-s.stop:
				return
			case <-t.C:
				s.Cleanup()
			}
		}
	}()
}

// Close stops the janitor goroutine if one is running.
func (s *MemoryStore) Close() error {
	if s.stop != nil {
		close(s.stop)
		<-s.done
		s.stop = nil
	}
	return nil
}
