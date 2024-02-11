package store

import (
	"context"
	"sync"
)

// MemStore is an in-memory Store, mostly for tests and local runs.
type MemStore struct {
	mu      sync.RWMutex
	records map[string]Record
	dedup   map[string]string
	closed  bool
}

// NewMemStore returns an empty in-memory store.
func NewMemStore() *MemStore {
	return &MemStore{
		records: make(map[string]Record),
		dedup:   make(map[string]string),
	}
}

func (m *MemStore) Put(_ context.Context, rec Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrClosed
	}
	m.records[rec.Token] = rec
	return nil
}

func (m *MemStore) Get(_ context.Context, token string) (Record, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return Record{}, false, ErrClosed
	}
	rec, ok := m.records[token]
	return rec, ok, nil
}

func (m *MemStore) PutDedup(_ context.Context, hmac, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrClosed
	}
	m.dedup[hmac] = token
	return nil
}

func (m *MemStore) GetDedup(_ context.Context, hmac string) (string, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return "", false, ErrClosed
	}
	token, ok := m.dedup[hmac]
	return token, ok, nil
}

func (m *MemStore) Ping(_ context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return ErrClosed
	}
	return nil
}

func (m *MemStore) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}
