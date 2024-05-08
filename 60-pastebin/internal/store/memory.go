package store

import (
	"context"
	"sync"
	"time"

	"pastebin/internal/paste"
)

// MemStore is an in-memory Store for tests and development.
type MemStore struct {
	mu     sync.RWMutex
	pastes map[string]paste.Paste
}

// NewMemStore returns an empty MemStore.
func NewMemStore() *MemStore {
	return &MemStore{pastes: make(map[string]paste.Paste)}
}

// Create implements Store.
func (s *MemStore) Create(ctx context.Context, p paste.Paste) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.pastes[p.ID]; ok {
		return ErrConflict
	}
	s.pastes[p.ID] = p
	return nil
}

// Get implements Store.
func (s *MemStore) Get(ctx context.Context, id string) (paste.Paste, error) {
	if err := ctx.Err(); err != nil {
		return paste.Paste{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.pastes[id]
	if !ok {
		return paste.Paste{}, ErrNotFound
	}
	return p, nil
}

// Delete implements Store.
func (s *MemStore) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.pastes[id]; !ok {
		return ErrNotFound
	}
	delete(s.pastes, id)
	return nil
}

// DeleteExpired implements Store.
func (s *MemStore) DeleteExpired(ctx context.Context, now time.Time) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for id, p := range s.pastes {
		if p.IsExpired(now) {
			delete(s.pastes, id)
			n++
		}
	}
	return n, nil
}

// Ping implements Store.
func (s *MemStore) Ping(ctx context.Context) error { return ctx.Err() }

// Close implements Store.
func (s *MemStore) Close() error { return nil }
