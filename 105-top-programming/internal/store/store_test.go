package store

import (
	"sync"
	"testing"

	"github.com/codingchallenges/topstories/internal/model"
)

func TestUpsertDedupHighestScore(t *testing.T) {
	s := New()
	s.Upsert(model.Story{ID: 1, Title: "a", Score: 10})
	s.Upsert(model.Story{ID: 1, Title: "a", Score: 50}) // higher wins
	s.Upsert(model.Story{ID: 1, Title: "a", Score: 30}) // lower ignored
	if s.Len() != 1 {
		t.Fatalf("expected 1 story, got %d", s.Len())
	}
	all := s.All()
	if all[0].Score != 50 {
		t.Fatalf("expected highest score 50, got %d", all[0].Score)
	}
}

func TestTopOrdering(t *testing.T) {
	s := New()
	s.Upsert(model.Story{ID: 1, Score: 10})
	s.Upsert(model.Story{ID: 2, Score: 80})
	s.Upsert(model.Story{ID: 3, Score: 80}) // tie, ID asc
	s.Upsert(model.Story{ID: 4, Score: 5})

	top := s.Top(3)
	want := []int64{2, 3, 1}
	if len(top) != 3 {
		t.Fatalf("Top(3): got %d", len(top))
	}
	for i, id := range want {
		if top[i].ID != id {
			t.Fatalf("Top order: got %d want %d at %d", top[i].ID, id, i)
		}
	}
}

func TestConcurrentUpsert(t *testing.T) {
	s := New()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			s.Upsert(model.Story{ID: int64(n % 10), Score: n})
		}(i)
	}
	wg.Wait()
	if s.Len() != 10 {
		t.Fatalf("expected 10 unique IDs, got %d", s.Len())
	}
}

func TestCategoryBackfillOnEqualScore(t *testing.T) {
	s := New()
	s.Upsert(model.Story{ID: 1, Score: 10, Category: ""})
	s.Upsert(model.Story{ID: 1, Score: 10, Category: "top"})
	if s.All()[0].Category != "top" {
		t.Fatal("category should backfill when previous empty and score equal")
	}
}
