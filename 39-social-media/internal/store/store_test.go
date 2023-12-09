package store

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"socialmedia/internal/model"

	"github.com/google/uuid"
)

func newTestStore(t *testing.T) *SQLite {
	t.Helper()
	// Each test gets a private in-memory database (unique DSN).
	s, err := Open(context.Background(), "file:"+uuid.NewString()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func mkPost(status model.Status, scheduledAt *time.Time, now time.Time) *model.Post {
	return &model.Post{
		ID:          uuid.NewString(),
		Author:      "acme",
		Body:        "hello world",
		Visibility:  model.VisibilityPublic,
		Target:      "mock",
		Status:      status,
		ScheduledAt: scheduledAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func TestCreateGetRoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	sched := now.Add(time.Hour)

	in := mkPost(model.StatusScheduled, &sched, now)
	in.ExternalID = "" // default
	if err := s.Create(ctx, in); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := s.Get(ctx, in.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Author != in.Author || got.Body != in.Body || got.Status != in.Status {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if got.ScheduledAt == nil || !got.ScheduledAt.Equal(sched) {
		t.Fatalf("scheduled_at mismatch: %v want %v", got.ScheduledAt, sched)
	}
	if !got.CreatedAt.Equal(now) {
		t.Fatalf("created_at mismatch: %v want %v", got.CreatedAt, now)
	}
}

func TestGetNotFound(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.Get(context.Background(), "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListFilterAndPaginate(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 3; i++ {
		p := mkPost(model.StatusQueued, nil, now.Add(time.Duration(i)*time.Second))
		if err := s.Create(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	p := mkPost(model.StatusScheduled, nil, now)
	p.Author = "other"
	if err := s.Create(ctx, p); err != nil {
		t.Fatal(err)
	}

	items, total, err := s.List(ctx, ListFilter{Status: "queued"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 3 || len(items) != 3 {
		t.Fatalf("status filter: total=%d len=%d", total, len(items))
	}

	items, total, err = s.List(ctx, ListFilter{Author: "other"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("author filter: total=%d len=%d", total, len(items))
	}

	items, total, err = s.List(ctx, ListFilter{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 || len(items) != 2 {
		t.Fatalf("pagination: total=%d len=%d", total, len(items))
	}
}

func TestClaimDueOnlySelectsDue(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)

	past := now.Add(-time.Minute)
	future := now.Add(time.Minute)

	due := mkPost(model.StatusScheduled, &past, now)
	notYet := mkPost(model.StatusScheduled, &future, now)
	alreadyQueued := mkPost(model.StatusQueued, &past, now)
	for _, p := range []*model.Post{due, notYet, alreadyQueued} {
		if err := s.Create(ctx, p); err != nil {
			t.Fatal(err)
		}
	}

	claimed, err := s.ClaimDue(ctx, now, 10)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != due.ID {
		t.Fatalf("expected only the due post, got %d: %+v", len(claimed), claimed)
	}
	if claimed[0].Status != model.StatusQueued {
		t.Fatalf("claimed post should be queued, got %s", claimed[0].Status)
	}

	// Re-claiming yields nothing (already queued).
	again, err := s.ClaimDue(ctx, now, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("expected no posts on second claim, got %d", len(again))
	}
}

func TestClaimDueExactlyOnceUnderConcurrency(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Minute)

	const n = 20
	for i := 0; i < n; i++ {
		if err := s.Create(ctx, mkPost(model.StatusScheduled, &past, now)); err != nil {
			t.Fatal(err)
		}
	}

	var mu sync.Mutex
	seen := map[string]int{}
	var wg sync.WaitGroup
	for w := 0; w < 5; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			claimed, err := s.ClaimDue(ctx, now, n)
			if err != nil {
				t.Errorf("claim: %v", err)
				return
			}
			mu.Lock()
			for _, p := range claimed {
				seen[p.ID]++
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	if len(seen) != n {
		t.Fatalf("claimed %d distinct posts, want %d", len(seen), n)
	}
	for id, c := range seen {
		if c != 1 {
			t.Fatalf("post %s claimed %d times (want exactly once)", id, c)
		}
	}
}

func TestEditGuardAndApply(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	sched := now.Add(time.Hour)

	p := mkPost(model.StatusScheduled, &sched, now)
	if err := s.Create(ctx, p); err != nil {
		t.Fatal(err)
	}

	// Editable: change body + reschedule.
	newBody := "edited body"
	newSched := now.Add(2 * time.Hour)
	updated, err := s.Edit(ctx, p.ID, EditFields{Body: &newBody, ScheduledAt: &newSched}, now)
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if updated.Body != newBody || !updated.ScheduledAt.Equal(newSched) {
		t.Fatalf("edit not applied: %+v", updated)
	}

	// Move to a non-editable status and confirm guard.
	if _, err := s.ClaimDue(ctx, now.Add(3*time.Hour), 10); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Edit(ctx, p.ID, EditFields{Body: &newBody}, now); !errors.Is(err, ErrNotEditable) {
		t.Fatalf("expected ErrNotEditable after claim, got %v", err)
	}
}

func TestPublishLifecycle(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)

	p := mkPost(model.StatusQueued, nil, now)
	if err := s.Create(ctx, p); err != nil {
		t.Fatal(err)
	}

	got, err := s.BeginPublish(ctx, p.ID, now)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if got.Status != model.StatusPublishing || got.Attempts != 1 {
		t.Fatalf("begin state: %+v", got)
	}

	// Second begin must fail (not queued anymore).
	if _, err := s.BeginPublish(ctx, p.ID, now); !errors.Is(err, ErrNotClaimable) {
		t.Fatalf("expected ErrNotClaimable, got %v", err)
	}

	fin, err := s.FinishPublish(ctx, p.ID, "ext-123", now)
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	if fin.Status != model.StatusPublished || fin.ExternalID != "ext-123" || fin.PublishedAt == nil {
		t.Fatalf("finish state: %+v", fin)
	}
}

func TestRetryThenFail(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)

	p := mkPost(model.StatusQueued, nil, now)
	if err := s.Create(ctx, p); err != nil {
		t.Fatal(err)
	}

	if _, err := s.BeginPublish(ctx, p.ID, now); err != nil {
		t.Fatal(err)
	}
	back, err := s.RetryPublish(ctx, p.ID, "boom", now)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if back.Status != model.StatusQueued || back.Error != "boom" {
		t.Fatalf("retry state: %+v", back)
	}

	if _, err := s.BeginPublish(ctx, p.ID, now); err != nil {
		t.Fatal(err)
	}
	failed, err := s.FailPublish(ctx, p.ID, "boom again", now)
	if err != nil {
		t.Fatalf("fail: %v", err)
	}
	if failed.Status != model.StatusFailed || failed.Attempts != 2 {
		t.Fatalf("fail state: %+v", failed)
	}
}

func TestCancel(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	sched := now.Add(time.Hour)

	p := mkPost(model.StatusScheduled, &sched, now)
	if err := s.Create(ctx, p); err != nil {
		t.Fatal(err)
	}
	canceled, err := s.Cancel(ctx, p.ID, now)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if canceled.Status != model.StatusCanceled {
		t.Fatalf("status: %s", canceled.Status)
	}
	// Cancelling again → not editable.
	if _, err := s.Cancel(ctx, p.ID, now); !errors.Is(err, ErrNotEditable) {
		t.Fatalf("expected ErrNotEditable, got %v", err)
	}
}
