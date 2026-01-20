package memory

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/codingchallenges/topstories/internal/model"
	"github.com/codingchallenges/topstories/internal/store"
)

func TestPublishConsumePipeline(t *testing.T) {
	bus := New(8)
	st := store.New()

	ctx, cancel := context.WithCancel(context.Background())
	var consumerErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		consumerErr = bus.Consume(ctx, func(s model.Story) error {
			st.Upsert(s)
			return nil
		})
	}()

	stories := []model.Story{
		{ID: 1, Title: "one", Score: 10, Source: "fake"},
		{ID: 2, Title: "two", Score: 20, Source: "fake"},
		{ID: 1, Title: "one", Score: 99, Source: "fake"}, // dedup, higher score
	}
	for _, s := range stories {
		if err := bus.Publish(context.Background(), s); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}

	// Wait until the store has both unique IDs with the higher score applied.
	waitFor(t, func() bool {
		return st.Len() == 2 && hasScore(st, 1, 99)
	})

	cancel()
	wg.Wait()
	if consumerErr != nil && consumerErr != context.Canceled {
		t.Fatalf("consumer err: %v", consumerErr)
	}
}

func TestPublishAfterCloseFails(t *testing.T) {
	bus := New(1)
	_ = bus.Close()
	err := bus.Publish(context.Background(), model.Story{ID: 1, Title: "x"})
	if err != ErrClosed {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
}

func TestPublishRespectsContext(t *testing.T) {
	bus := New(0) // unbuffered, no consumer -> publish blocks
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := bus.Publish(ctx, model.Story{ID: 1, Title: "x"})
	if err == nil {
		t.Fatal("expected context deadline error")
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("condition not met before deadline")
}

func hasScore(s *store.MemStore, id int64, score int) bool {
	for _, st := range s.All() {
		if st.ID == id {
			return st.Score == score
		}
	}
	return false
}
