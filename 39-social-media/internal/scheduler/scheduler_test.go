package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"socialmedia/internal/clock"
	"socialmedia/internal/model"
	"socialmedia/internal/queue"
	"socialmedia/internal/store"
)

func newStore(t *testing.T) *store.SQLite {
	t.Helper()
	s, err := store.Open(context.Background(), "file:"+uuid.NewString()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func mkScheduled(t *testing.T, s *store.SQLite, at time.Time, now time.Time) *model.Post {
	t.Helper()
	p := &model.Post{
		ID:          uuid.NewString(),
		Author:      "acme",
		Body:        "hi",
		Visibility:  model.VisibilityPublic,
		Target:      "mock",
		Status:      model.StatusScheduled,
		ScheduledAt: &at,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.Create(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	return p
}

func drain(q *queue.Memory) map[string]bool {
	seen := map[string]bool{}
	ctx := context.Background()
	for q.Len() > 0 {
		m, err := q.Dequeue(ctx)
		if err != nil {
			break
		}
		seen[m.PostID] = true
	}
	return seen
}

func TestSchedulerDispatchesDueOnly(t *testing.T) {
	s := newStore(t)
	q := queue.NewMemory(16)
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	clk := clock.NewFake(now)

	due := mkScheduled(t, s, now.Add(-time.Second), now)
	exactlyNow := mkScheduled(t, s, now, now)
	future := mkScheduled(t, s, now.Add(time.Hour), now)

	sch := New(Config{Store: s, Queue: q, Clock: clk})

	n, err := sch.Tick(context.Background())
	if err != nil {
		t.Fatalf("tick: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 dispatched (due + exactly-now), got %d", n)
	}
	seen := drain(q)
	if !seen[due.ID] || !seen[exactlyNow.ID] {
		t.Fatalf("due posts not dispatched: %v", seen)
	}
	if seen[future.ID] {
		t.Fatalf("future post should not be dispatched")
	}

	// The dispatched posts are now queued in the store.
	got, _ := s.Get(context.Background(), due.ID)
	if got.Status != model.StatusQueued {
		t.Fatalf("due post status = %s, want queued", got.Status)
	}
	gotFuture, _ := s.Get(context.Background(), future.ID)
	if gotFuture.Status != model.StatusScheduled {
		t.Fatalf("future post status = %s, want scheduled", gotFuture.Status)
	}
}

func TestSchedulerDispatchesAfterClockAdvances(t *testing.T) {
	s := newStore(t)
	q := queue.NewMemory(16)
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	clk := clock.NewFake(now)
	sch := New(Config{Store: s, Queue: q, Clock: clk})

	p := mkScheduled(t, s, now.Add(30*time.Minute), now)

	// Not due yet.
	if n, _ := sch.Tick(context.Background()); n != 0 {
		t.Fatalf("expected nothing dispatched before due time, got %d", n)
	}

	// Advance the fake clock past the schedule — no real waiting.
	clk.Advance(31 * time.Minute)
	if n, _ := sch.Tick(context.Background()); n != 1 {
		t.Fatalf("expected 1 dispatched after clock advance, got %d", n)
	}
	if !drain(q)[p.ID] {
		t.Fatalf("post not dispatched after becoming due")
	}
}

func TestSchedulerSkipsCanceledPost(t *testing.T) {
	s := newStore(t)
	q := queue.NewMemory(16)
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	clk := clock.NewFake(now)
	sch := New(Config{Store: s, Queue: q, Clock: clk})

	p := mkScheduled(t, s, now.Add(-time.Second), now)
	if _, err := s.Cancel(context.Background(), p.ID, now); err != nil {
		t.Fatal(err)
	}

	if n, _ := sch.Tick(context.Background()); n != 0 {
		t.Fatalf("canceled post must not be dispatched, dispatched %d", n)
	}
}

func TestSchedulerRunStopsOnContextCancel(t *testing.T) {
	s := newStore(t)
	q := queue.NewMemory(4)
	clk := clock.NewFake(time.Now().UTC())
	sch := New(Config{Store: s, Queue: q, Clock: clk, Interval: time.Millisecond})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- sch.Run(ctx) }()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after context cancel")
	}
}
