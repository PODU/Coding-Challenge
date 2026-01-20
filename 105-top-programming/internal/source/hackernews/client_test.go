package hackernews

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/codingchallenges/topstories/internal/source"
)

// fixtureServer serves captured HN JSON from internal/source/fixtures. It maps
// the real Firebase paths onto flat fixture filenames:
//
//	/v0/topstories.json   -> fixtures/topstories.json
//	/v0/item/101.json     -> fixtures/item_101.json
func fixtureServer(t *testing.T) *httptest.Server {
	t.Helper()
	dir := filepath.Join("..", "fixtures")
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/v0/")
		var file string
		switch {
		case strings.HasPrefix(p, "item/"):
			id := strings.TrimSuffix(strings.TrimPrefix(p, "item/"), ".json")
			file = filepath.Join(dir, "item_"+id+".json")
		case p == "fivehundred.json":
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		default:
			file = filepath.Join(dir, p)
		}
		data, err := os.ReadFile(file)
		if err != nil {
			// HN returns literal null for unknown items.
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("null"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	}))
}

func newTestClient(t *testing.T, base string) *Client {
	return New(WithBaseURL(base+"/v0"), WithRetries(1, time.Millisecond))
}

func TestStoryIDs(t *testing.T) {
	srv := fixtureServer(t)
	defer srv.Close()
	c := newTestClient(t, srv.URL)

	ids, err := c.StoryIDs(context.Background(), source.CatTop)
	if err != nil {
		t.Fatalf("StoryIDs: %v", err)
	}
	want := []int64{101, 102, 103, 104, 105}
	if len(ids) != len(want) {
		t.Fatalf("got %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("got %v, want %v", ids, want)
		}
	}
}

func TestFetchMapping(t *testing.T) {
	srv := fixtureServer(t)
	defer srv.Close()
	c := newTestClient(t, srv.URL)

	st, err := c.Fetch(context.Background(), 101)
	if err != nil {
		t.Fatalf("Fetch 101: %v", err)
	}
	if st.Author != "alice" {
		t.Errorf("by->Author: got %q", st.Author)
	}
	if st.Score != 250 {
		t.Errorf("score: got %d", st.Score)
	}
	if st.Source != "hackernews" {
		t.Errorf("source: got %q", st.Source)
	}
	wantTime := time.Unix(1700000000, 0).UTC()
	if !st.Timestamp.Equal(wantTime) {
		t.Errorf("timestamp: got %v want %v", st.Timestamp, wantTime)
	}
	if st.Timestamp.Location() != time.UTC {
		t.Errorf("timestamp not UTC: %v", st.Timestamp.Location())
	}
}

func TestFetchDefaults(t *testing.T) {
	srv := fixtureServer(t)
	defer srv.Close()
	c := newTestClient(t, srv.URL)

	// item 104 has no "by" and no score -> defaults.
	st, err := c.Fetch(context.Background(), 104)
	if err != nil {
		t.Fatalf("Fetch 104: %v", err)
	}
	if st.Author != "unknown" {
		t.Errorf("missing by should default to unknown, got %q", st.Author)
	}
	if st.Score != 0 {
		t.Errorf("missing score should default to 0, got %d", st.Score)
	}
	if st.URL != "" {
		t.Errorf("Ask HN should have empty URL, got %q", st.URL)
	}
}

func TestFetchDeletedDeadSkipped(t *testing.T) {
	srv := fixtureServer(t)
	defer srv.Close()
	c := newTestClient(t, srv.URL)

	for _, id := range []int64{102, 103} {
		_, err := c.Fetch(context.Background(), id)
		if !source.IsSkip(err) {
			t.Errorf("id %d: expected skip, got %v", id, err)
		}
	}
}

func TestFetchMissingItem(t *testing.T) {
	srv := fixtureServer(t)
	defer srv.Close()
	c := newTestClient(t, srv.URL)

	_, err := c.Fetch(context.Background(), 999999)
	if !source.IsSkip(err) {
		t.Errorf("missing item should skip, got %v", err)
	}
}

func TestServerErrorRetried(t *testing.T) {
	srv := fixtureServer(t)
	defer srv.Close()
	c := New(WithBaseURL(srv.URL+"/v0"), WithRetries(2, time.Millisecond))

	// fivehundred.json always 500s; should exhaust retries and error.
	err := c.getJSON(context.Background(), srv.URL+"/v0/fivehundred.json", new(any))
	if err == nil {
		t.Fatal("expected error from persistent 500")
	}
}
