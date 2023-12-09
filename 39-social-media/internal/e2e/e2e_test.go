// Package e2e exercises the whole pipeline in one process — store, in-memory
// queue, scheduler (fake clock), publisher worker pool and mock provider —
// with no network, no Docker and no real waiting on scheduled times.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"socialmedia/internal/api"
	"socialmedia/internal/clock"
	"socialmedia/internal/model"
	"socialmedia/internal/provider"
	"socialmedia/internal/publisher"
	"socialmedia/internal/queue"
	"socialmedia/internal/scheduler"
	"socialmedia/internal/store"
)

type harness struct {
	ts    *httptest.Server
	clk   *clock.Fake
	mock  *provider.Mock
	store store.Store
	sched *scheduler.Scheduler
}

func boot(t *testing.T) *harness {
	t.Helper()
	st, err := store.Open(context.Background(), "file:"+uuid.NewString()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	q := queue.NewMemory(64)
	clk := clock.NewFake(time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC))
	mock := provider.NewMock()
	reg := provider.NewRegistry(mock)

	pub := publisher.New(publisher.Config{
		Store: st, Queue: q, Registry: reg, Clock: clk, Workers: 3,
		Backoff: func(int) time.Duration { return 0 },
	})
	sch := scheduler.New(scheduler.Config{Store: st, Queue: q, Clock: clk})

	ctx, cancel := context.WithCancel(context.Background())
	pub.Start(ctx)

	srv := api.NewServer(api.Config{Store: st, Queue: q, Registry: reg, Clock: clk})
	ts := httptest.NewServer(srv.Handler())

	t.Cleanup(func() {
		ts.Close()
		cancel()
		q.Close()
		pub.Wait()
		_ = st.Close()
	})
	return &harness{ts: ts, clk: clk, mock: mock, store: st, sched: sch}
}

func (h *harness) create(t *testing.T, body map[string]any) model.Post {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := h.ts.Client().Post(h.ts.URL+"/api/v1/posts", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("create status %d", resp.StatusCode)
	}
	var p model.Post
	json.NewDecoder(resp.Body).Decode(&p)
	return p
}

func (h *harness) waitPublished(t *testing.T, id string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		p, err := h.store.Get(context.Background(), id)
		if err == nil && p.Status == model.StatusPublished {
			return
		}
		time.Sleep(time.Millisecond)
	}
	p, _ := h.store.Get(context.Background(), id)
	t.Fatalf("post %s not published; status=%s err=%s", id, p.Status, p.Error)
}

func TestE2EImmediatePublish(t *testing.T) {
	h := boot(t)
	p := h.create(t, map[string]any{"author": "acme", "body": "ship it", "target": "mock"})
	h.waitPublished(t, p.ID)
	if h.mock.CallCount() != 1 {
		t.Fatalf("expected 1 publish call, got %d", h.mock.CallCount())
	}
}

func TestE2EScheduledPublishViaClockAdvance(t *testing.T) {
	h := boot(t)
	future := h.clk.Now().Add(2 * time.Hour)
	p := h.create(t, map[string]any{
		"author": "acme", "body": "scheduled ship", "target": "mock",
		"scheduled_at": future.Format(time.RFC3339),
	})

	// Before due: a tick dispatches nothing.
	if n, _ := h.sched.Tick(context.Background()); n != 0 {
		t.Fatalf("dispatched %d before due time", n)
	}

	// Advance the fake clock past the schedule, then tick — no real waiting.
	h.clk.Advance(3 * time.Hour)
	if n, _ := h.sched.Tick(context.Background()); n != 1 {
		t.Fatalf("expected 1 dispatched after clock advance, got %d", n)
	}
	h.waitPublished(t, p.ID)
	if h.mock.CallCount() != 1 {
		t.Fatalf("expected 1 publish call, got %d", h.mock.CallCount())
	}
}

func TestE2EEditThenPublish(t *testing.T) {
	h := boot(t)
	future := h.clk.Now().Add(time.Hour)
	p := h.create(t, map[string]any{
		"author": "acme", "body": "draft text", "target": "mock",
		"scheduled_at": future.Format(time.RFC3339),
	})

	// Edit the body before it is due.
	b, _ := json.Marshal(map[string]any{"body": "final text"})
	req, _ := newPatch(h.ts.URL+"/api/v1/posts/"+p.ID, b)
	resp, err := h.ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("patch status %d", resp.StatusCode)
	}

	// Make it due and dispatch.
	h.clk.Advance(2 * time.Hour)
	if n, _ := h.sched.Tick(context.Background()); n != 1 {
		t.Fatalf("expected 1 dispatched, got %d", n)
	}
	h.waitPublished(t, p.ID)

	calls := h.mock.Calls()
	if len(calls) != 1 || calls[0].Body != "final text" {
		t.Fatalf("published body = %q, want edited 'final text'", calls)
	}
}

func newPatch(url string, body []byte) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}
