package pipeline

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/codingchallenges/topstories/internal/broker/memory"
	"github.com/codingchallenges/topstories/internal/model"
	"github.com/codingchallenges/topstories/internal/source"
	"github.com/codingchallenges/topstories/internal/store"
)

func TestDeduperScoreAware(t *testing.T) {
	d := NewDeduper()
	if !d.ShouldPublish(model.Story{ID: 1, Score: 10}) {
		t.Fatal("first publish should be allowed")
	}
	if d.ShouldPublish(model.Story{ID: 1, Score: 10}) {
		t.Fatal("same score should be deduped")
	}
	if !d.ShouldPublish(model.Story{ID: 1, Score: 20}) {
		t.Fatal("higher score should re-publish")
	}
	if d.ShouldPublish(model.Story{ID: 1, Score: 15}) {
		t.Fatal("lower score should be deduped")
	}
}

// TestFullPipeline exercises aggregator -> producer -> in-memory bus ->
// consumer -> store, fully offline.
func TestFullPipeline(t *testing.T) {
	src := sampleSource()
	agg := New(src, 4)
	stories, err := agg.Collect(context.Background(), source.CatTop, source.CatBest)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}

	bus := memory.New(16)
	st := store.New()

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = bus.Consume(ctx, func(s model.Story) error {
			st.Upsert(s)
			return nil
		})
	}()

	n := PublishAll(context.Background(), bus, NewDeduper(), stories, agg.log)
	if n != len(stories) {
		t.Fatalf("published %d, expected %d", n, len(stories))
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && st.Len() < len(stories) {
		time.Sleep(2 * time.Millisecond)
	}
	cancel()
	wg.Wait()

	if st.Len() != 4 {
		t.Fatalf("store has %d stories, want 4", st.Len())
	}
	top := st.Top(1)
	if top[0].ID != 4 || top[0].Score != 80 {
		t.Fatalf("top story wrong: %+v", top[0])
	}
}
