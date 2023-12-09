package publisher

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"socialmedia/internal/clock"
	"socialmedia/internal/model"
	"socialmedia/internal/provider"
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

func mkQueued(t *testing.T, s *store.SQLite) *model.Post {
	t.Helper()
	now := time.Now().UTC()
	p := &model.Post{
		ID:         uuid.NewString(),
		Author:     "acme",
		Body:       "hi",
		Visibility: model.VisibilityPublic,
		Target:     "mock",
		Status:     model.StatusQueued,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.Create(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	return p
}

// waitForStatus polls the store (logical, not wall-clock dependent) until the
// post reaches want or the deadline elapses.
func waitForStatus(t *testing.T, s store.Store, id string, want model.Status) *model.Post {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		p, err := s.Get(context.Background(), id)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if p.Status == want {
			return p
		}
		time.Sleep(time.Millisecond)
	}
	p, _ := s.Get(context.Background(), id)
	t.Fatalf("post %s did not reach %q (last status %q, err %q)", id, want, p.Status, p.Error)
	return nil
}

func startPublisher(t *testing.T, cfg Config) (context.CancelFunc, *queue.Memory) {
	t.Helper()
	if cfg.Backoff == nil {
		cfg.Backoff = func(int) time.Duration { return 0 } // hermetic: no real waiting
	}
	q := cfg.Queue.(*queue.Memory)
	pub := New(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	pub.Start(ctx)
	t.Cleanup(func() {
		cancel()
		q.Close()
		pub.Wait()
	})
	return cancel, q
}

func TestPublisherPublishesSuccessfully(t *testing.T) {
	s := newStore(t)
	q := queue.NewMemory(8)
	mock := provider.NewMock()
	reg := provider.NewRegistry(mock)

	startPublisher(t, Config{Store: s, Queue: q, Registry: reg, Clock: clock.System{}, Workers: 2})

	p := mkQueued(t, s)
	if err := q.Enqueue(context.Background(), queue.Message{PostID: p.ID}); err != nil {
		t.Fatal(err)
	}

	got := waitForStatus(t, s, p.ID, model.StatusPublished)
	if got.ExternalID == "" || got.PublishedAt == nil {
		t.Fatalf("published post missing external_id/published_at: %+v", got)
	}
	if mock.CallCount() != 1 {
		t.Fatalf("expected 1 provider call, got %d", mock.CallCount())
	}
}

func TestPublisherRetriesThenSucceeds(t *testing.T) {
	s := newStore(t)
	q := queue.NewMemory(8)
	mock := provider.NewMock()
	mock.FailTimes = 2 // first two attempts fail, third succeeds
	reg := provider.NewRegistry(mock)

	startPublisher(t, Config{Store: s, Queue: q, Registry: reg, Clock: clock.System{}, Workers: 1, MaxAttempts: 5})

	p := mkQueued(t, s)
	_ = q.Enqueue(context.Background(), queue.Message{PostID: p.ID})

	got := waitForStatus(t, s, p.ID, model.StatusPublished)
	if got.Attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", got.Attempts)
	}
}

func TestPublisherFailsAfterMaxAttempts(t *testing.T) {
	s := newStore(t)
	q := queue.NewMemory(8)
	mock := provider.NewMock()
	mock.FailAlways = true
	reg := provider.NewRegistry(mock)

	startPublisher(t, Config{Store: s, Queue: q, Registry: reg, Clock: clock.System{}, Workers: 1, MaxAttempts: 3})

	p := mkQueued(t, s)
	_ = q.Enqueue(context.Background(), queue.Message{PostID: p.ID})

	got := waitForStatus(t, s, p.ID, model.StatusFailed)
	if got.Attempts != 3 {
		t.Fatalf("expected 3 attempts before failing, got %d", got.Attempts)
	}
	if got.Error == "" {
		t.Fatalf("failed post should record an error")
	}
}

func TestPublisherAuthErrorIsNotRetried(t *testing.T) {
	s := newStore(t)
	q := queue.NewMemory(8)
	mock := provider.NewMock()
	mock.FailAuth = true
	reg := provider.NewRegistry(mock)

	startPublisher(t, Config{Store: s, Queue: q, Registry: reg, Clock: clock.System{}, Workers: 1, MaxAttempts: 5})

	p := mkQueued(t, s)
	_ = q.Enqueue(context.Background(), queue.Message{PostID: p.ID})

	got := waitForStatus(t, s, p.ID, model.StatusFailed)
	if got.Attempts != 1 {
		t.Fatalf("auth failure must not retry; expected 1 attempt, got %d", got.Attempts)
	}
}

func TestPublisherDropsCanceledMessage(t *testing.T) {
	s := newStore(t)
	q := queue.NewMemory(8)
	mock := provider.NewMock()
	reg := provider.NewRegistry(mock)

	startPublisher(t, Config{Store: s, Queue: q, Registry: reg, Clock: clock.System{}, Workers: 1})

	// Post is canceled (not queued) → worker must drop it without publishing.
	now := time.Now().UTC()
	p := &model.Post{
		ID: uuid.NewString(), Author: "a", Body: "b", Visibility: model.VisibilityPublic,
		Target: "mock", Status: model.StatusCanceled, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.Create(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	_ = q.Enqueue(context.Background(), queue.Message{PostID: p.ID})

	// Give the worker time to consume and drop the message.
	time.Sleep(50 * time.Millisecond)
	if mock.AttemptCount() != 0 {
		t.Fatalf("canceled post must not be published, got %d attempts", mock.AttemptCount())
	}
	got, _ := s.Get(context.Background(), p.ID)
	if got.Status != model.StatusCanceled {
		t.Fatalf("status changed to %s", got.Status)
	}
}

func TestPublisherIdempotentRedelivery(t *testing.T) {
	s := newStore(t)
	q := queue.NewMemory(8)
	mock := provider.NewMock()
	reg := provider.NewRegistry(mock)

	startPublisher(t, Config{Store: s, Queue: q, Registry: reg, Clock: clock.System{}, Workers: 1})

	p := mkQueued(t, s)
	_ = q.Enqueue(context.Background(), queue.Message{PostID: p.ID})
	waitForStatus(t, s, p.ID, model.StatusPublished)

	// Re-deliver the same message: it is already published, so BeginPublish
	// rejects it and no second provider call happens.
	_ = q.Enqueue(context.Background(), queue.Message{PostID: p.ID})
	time.Sleep(50 * time.Millisecond)
	if mock.CallCount() != 1 {
		t.Fatalf("re-delivery double-published: %d calls", mock.CallCount())
	}
}
