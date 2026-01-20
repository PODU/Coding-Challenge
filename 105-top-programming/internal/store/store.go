// Package store provides a thread-safe in-memory story store.
package store

import (
	"sort"
	"sync"

	"github.com/codingchallenges/topstories/internal/model"
)

// Store is the read/write interface backing the REST API.
type Store interface {
	Upsert(s model.Story) // dedup by ID, keep highest score
	All() []model.Story
	Top(n int) []model.Story
	Len() int
}

// MemStore is a mutex-guarded map implementation of Store.
type MemStore struct {
	mu      sync.RWMutex
	stories map[int64]model.Story
}

var _ Store = (*MemStore)(nil)

// New builds an empty MemStore.
func New() *MemStore {
	return &MemStore{stories: make(map[int64]model.Story)}
}

// Upsert inserts or updates the story, keeping the highest score on collision.
func (m *MemStore) Upsert(s model.Story) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if prev, ok := m.stories[s.ID]; ok && prev.Score >= s.Score {
		// Keep the existing story but take the category if it had none.
		if prev.Category == "" && s.Category != "" {
			prev.Category = s.Category
			m.stories[s.ID] = prev
		}
		return
	}
	m.stories[s.ID] = s
}

// All returns a snapshot of every story, sorted by score desc then ID asc.
func (m *MemStore) All() []model.Story {
	m.mu.RLock()
	out := make([]model.Story, 0, len(m.stories))
	for _, s := range m.stories {
		out = append(out, s)
	}
	m.mu.RUnlock()
	sortStories(out)
	return out
}

// Top returns the n highest-scored stories.
func (m *MemStore) Top(n int) []model.Story {
	all := m.All()
	if n > 0 && n < len(all) {
		all = all[:n]
	}
	return all
}

// Len returns the number of stored stories.
func (m *MemStore) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.stories)
}

func sortStories(s []model.Story) {
	sort.SliceStable(s, func(i, j int) bool {
		if s[i].Score != s[j].Score {
			return s[i].Score > s[j].Score
		}
		return s[i].ID < s[j].ID
	})
}
