package store

import (
	"context"
	"sync"
	"testing"
	"time"

	"ratelimiter/internal/clock"
)

// a trivial counting step used to exercise the store.
func countStep(st *State) Decision {
	st.Count++
	return Decision{Allowed: true, Remaining: st.Count}
}

func TestMemoryStore_PersistsState(t *testing.T) {
	clk := clock.NewFakeClock(time.Unix(0, 0))
	s := NewMemoryStore(clk, 0)
	defer s.Close()

	for i := 1; i <= 3; i++ {
		dec, err := s.Do(context.Background(), "k", countStep)
		if err != nil {
			t.Fatalf("Do: %v", err)
		}
		if dec.Remaining != i {
			t.Fatalf("expected count %d, got %d", i, dec.Remaining)
		}
	}
}

func TestMemoryStore_JanitorEvictsIdle(t *testing.T) {
	clk := clock.NewFakeClock(time.Unix(0, 0))
	s := NewMemoryStore(clk, 30*time.Second)
	defer s.Close()

	_, _ = s.Do(context.Background(), "a", countStep)
	clk.Advance(10 * time.Second)
	_, _ = s.Do(context.Background(), "b", countStep)

	if s.Len() != 2 {
		t.Fatalf("expected 2 keys, got %d", s.Len())
	}

	// Advance so "a" (last touched at t=0) is idle > 30s but "b" (t=10s) is not.
	clk.Advance(25 * time.Second) // now t=35s
	if evicted := s.Cleanup(); evicted != 1 {
		t.Fatalf("expected 1 eviction, got %d", evicted)
	}
	if s.Len() != 1 {
		t.Fatalf("expected 1 key after cleanup, got %d", s.Len())
	}

	// Advance further so "b" also becomes idle.
	clk.Advance(30 * time.Second)
	if evicted := s.Cleanup(); evicted != 1 {
		t.Fatalf("expected 1 more eviction, got %d", evicted)
	}
	if s.Len() != 0 {
		t.Fatalf("expected 0 keys, got %d", s.Len())
	}
}

func TestMemoryStore_ConcurrentAtomicity(t *testing.T) {
	clk := clock.NewFakeClock(time.Unix(0, 0))
	s := NewMemoryStore(clk, 0)
	defer s.Close()

	const n = 500
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, _ = s.Do(context.Background(), "k", countStep)
		}()
	}
	wg.Wait()

	dec, _ := s.Do(context.Background(), "k", func(st *State) Decision {
		return Decision{Remaining: st.Count}
	})
	if dec.Remaining != n {
		t.Fatalf("expected %d increments, got %d (lost updates)", n, dec.Remaining)
	}
}
