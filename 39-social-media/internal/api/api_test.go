package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"socialmedia/internal/clock"
	"socialmedia/internal/model"
	"socialmedia/internal/provider"
	"socialmedia/internal/queue"
	"socialmedia/internal/store"

	"github.com/google/uuid"
)

func newTestServer(t *testing.T) (*httptest.Server, *clock.Fake, *queue.Memory, store.Store) {
	t.Helper()
	st, err := store.Open(context.Background(), "file:"+uuid.NewString()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	q := queue.NewMemory(32)
	clk := clock.NewFake(time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC))
	reg := provider.NewRegistry(provider.NewMock())
	srv := NewServer(Config{Store: st, Queue: q, Registry: reg, Clock: clk})

	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, clk, q, st
}

func doJSON(t *testing.T, method, url string, body any) *http.Response {
	t.Helper()
	var r *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	} else {
		r = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func decodePost(t *testing.T, resp *http.Response) model.Post {
	t.Helper()
	defer resp.Body.Close()
	var p model.Post
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return p
}

func TestCreateImmediate(t *testing.T) {
	ts, _, q, _ := newTestServer(t)

	resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/posts", map[string]any{
		"author": "acme", "body": "hello", "target": "mock",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	p := decodePost(t, resp)
	if p.Status != model.StatusQueued {
		t.Fatalf("immediate post status = %s, want queued", p.Status)
	}
	if p.Visibility != model.VisibilityPublic {
		t.Fatalf("default visibility = %s, want public", p.Visibility)
	}
	if q.Len() != 1 {
		t.Fatalf("immediate post not enqueued (len=%d)", q.Len())
	}
}

func TestCreateScheduled(t *testing.T) {
	ts, clk, q, _ := newTestServer(t)
	future := clk.Now().Add(time.Hour)

	resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/posts", map[string]any{
		"author": "acme", "body": "later", "target": "mock",
		"scheduled_at": future.Format(time.RFC3339),
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	p := decodePost(t, resp)
	if p.Status != model.StatusScheduled {
		t.Fatalf("status = %s, want scheduled", p.Status)
	}
	if q.Len() != 0 {
		t.Fatalf("scheduled post must not be enqueued immediately")
	}
}

func TestCreateValidationErrors(t *testing.T) {
	ts, clk, _, _ := newTestServer(t)
	past := clk.Now().Add(-time.Hour)

	cases := []map[string]any{
		{"author": "", "body": "x", "target": "mock"},                                             // missing author
		{"author": "a", "body": "", "target": "mock"},                                             // missing body
		{"author": "a", "body": "x", "target": "nope"},                                            // unknown target
		{"author": "a", "body": "x", "target": "mock", "visibility": "loud"},                      // bad visibility
		{"author": "a", "body": "x", "target": "mock", "scheduled_at": past.Format(time.RFC3339)}, // past schedule
	}
	for i, c := range cases {
		resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/posts", c)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("case %d: status = %d, want 400", i, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestGetAndList(t *testing.T) {
	ts, _, _, _ := newTestServer(t)

	created := decodePost(t, doJSON(t, http.MethodPost, ts.URL+"/api/v1/posts", map[string]any{
		"author": "acme", "body": "hello", "target": "mock",
	}))

	// Get by id.
	resp := doJSON(t, http.MethodGet, ts.URL+"/api/v1/posts/"+created.ID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Get missing → 404.
	resp = doJSON(t, http.MethodGet, ts.URL+"/api/v1/posts/does-not-exist", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("missing get status = %d, want 404", resp.StatusCode)
	}
	resp.Body.Close()

	// List with filter.
	resp = doJSON(t, http.MethodGet, ts.URL+"/api/v1/posts?status=queued", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d", resp.StatusCode)
	}
	var lr listResponse
	json.NewDecoder(resp.Body).Decode(&lr)
	resp.Body.Close()
	if lr.Total != 1 || len(lr.Items) != 1 {
		t.Fatalf("list total=%d len=%d", lr.Total, len(lr.Items))
	}
}

func TestPatchScheduledAnd409(t *testing.T) {
	ts, clk, _, _ := newTestServer(t)
	future := clk.Now().Add(time.Hour)

	// Editable scheduled post.
	scheduled := decodePost(t, doJSON(t, http.MethodPost, ts.URL+"/api/v1/posts", map[string]any{
		"author": "acme", "body": "v1", "target": "mock",
		"scheduled_at": future.Format(time.RFC3339),
	}))
	resp := doJSON(t, http.MethodPatch, ts.URL+"/api/v1/posts/"+scheduled.ID, map[string]any{"body": "v2"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch status = %d, want 200", resp.StatusCode)
	}
	updated := decodePost(t, resp)
	if updated.Body != "v2" {
		t.Fatalf("patch did not apply: %q", updated.Body)
	}

	// Immediate post is queued (not editable) → 409.
	immediate := decodePost(t, doJSON(t, http.MethodPost, ts.URL+"/api/v1/posts", map[string]any{
		"author": "acme", "body": "now", "target": "mock",
	}))
	resp = doJSON(t, http.MethodPatch, ts.URL+"/api/v1/posts/"+immediate.ID, map[string]any{"body": "x"})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("patch queued status = %d, want 409", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestDeleteCancelsScheduled(t *testing.T) {
	ts, clk, _, st := newTestServer(t)
	future := clk.Now().Add(time.Hour)

	scheduled := decodePost(t, doJSON(t, http.MethodPost, ts.URL+"/api/v1/posts", map[string]any{
		"author": "acme", "body": "later", "target": "mock",
		"scheduled_at": future.Format(time.RFC3339),
	}))
	resp := doJSON(t, http.MethodDelete, ts.URL+"/api/v1/posts/"+scheduled.ID, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", resp.StatusCode)
	}
	resp.Body.Close()

	got, _ := st.Get(context.Background(), scheduled.ID)
	if got.Status != model.StatusCanceled {
		t.Fatalf("status = %s, want canceled", got.Status)
	}
}

func TestPublishNow(t *testing.T) {
	ts, clk, q, _ := newTestServer(t)
	future := clk.Now().Add(time.Hour)

	scheduled := decodePost(t, doJSON(t, http.MethodPost, ts.URL+"/api/v1/posts", map[string]any{
		"author": "acme", "body": "later", "target": "mock",
		"scheduled_at": future.Format(time.RFC3339),
	}))
	resp := doJSON(t, http.MethodPost, ts.URL+"/api/v1/posts/"+scheduled.ID+"/publish", nil)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("publish-now status = %d, want 202", resp.StatusCode)
	}
	p := decodePost(t, resp)
	if p.Status != model.StatusQueued {
		t.Fatalf("status = %s, want queued", p.Status)
	}
	if q.Len() != 1 {
		t.Fatalf("publish-now did not enqueue")
	}
}

func TestHealthz(t *testing.T) {
	ts, _, _, _ := newTestServer(t)
	resp := doJSON(t, http.MethodGet, ts.URL+"/healthz", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
}
