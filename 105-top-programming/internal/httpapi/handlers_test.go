package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/codingchallenges/topstories/internal/model"
	"github.com/codingchallenges/topstories/internal/store"
)

func seeded() *store.MemStore {
	s := store.New()
	b := time.Date(2023, 11, 15, 0, 0, 0, 0, time.UTC)
	s.Upsert(model.Story{ID: 1, Title: "Rust safety", Score: 300, Type: "story", Timestamp: b, Source: "hn"})
	s.Upsert(model.Story{ID: 2, Title: "Go generics", Score: 150, Type: "story", Timestamp: b.Add(time.Hour), Source: "hn"})
	s.Upsert(model.Story{ID: 3, Title: "Hiring job", Score: 5, Type: "job", Timestamp: b.Add(2 * time.Hour), Source: "hn"})
	return s
}

func doGet(t *testing.T, h http.Handler, url string) (*http.Response, []model.Story) {
	t.Helper()
	srv := httptest.NewServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL + url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	var out []model.Story
	if resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode: %v", err)
		}
	}
	resp.Body.Close()
	return resp, out
}

func TestHealthz(t *testing.T) {
	h := New(seeded(), nil)
	srv := httptest.NewServer(h)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status %d", resp.StatusCode)
	}
}

func TestStoriesAll(t *testing.T) {
	resp, out := doGet(t, New(seeded(), nil), "/stories")
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if len(out) != 3 {
		t.Fatalf("expected 3, got %d", len(out))
	}
	// default sort score desc
	if out[0].ID != 1 {
		t.Fatalf("expected top story id 1, got %d", out[0].ID)
	}
}

func TestStoriesMinScore(t *testing.T) {
	_, out := doGet(t, New(seeded(), nil), "/stories?min_score=100")
	if len(out) != 2 {
		t.Fatalf("min_score=100: got %d", len(out))
	}
}

func TestStoriesTypeFilter(t *testing.T) {
	_, out := doGet(t, New(seeded(), nil), "/stories?type=job")
	if len(out) != 1 || out[0].ID != 3 {
		t.Fatalf("type=job: got %v", out)
	}
}

func TestStoriesKeyword(t *testing.T) {
	_, out := doGet(t, New(seeded(), nil), "/stories?q=go")
	if len(out) != 1 || out[0].ID != 2 {
		t.Fatalf("q=go: got %v", out)
	}
}

func TestStoriesTimeWindow(t *testing.T) {
	// since = b+1h as unix seconds
	since := time.Date(2023, 11, 15, 1, 0, 0, 0, time.UTC).Unix()
	_, out := doGet(t, New(seeded(), nil), "/stories?since="+itoa(since))
	if len(out) != 2 {
		t.Fatalf("since filter: got %d", len(out))
	}
}

func TestStoriesPagination(t *testing.T) {
	_, out := doGet(t, New(seeded(), nil), "/stories?limit=1&offset=1")
	if len(out) != 1 || out[0].ID != 2 {
		t.Fatalf("pagination: got %v", out)
	}
}

func TestStoriesOffsetPastEndEmpty(t *testing.T) {
	resp, out := doGet(t, New(seeded(), nil), "/stories?offset=99")
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if len(out) != 0 {
		t.Fatalf("offset past end should be empty, got %d", len(out))
	}
}

func TestStoriesEmptyReturnsJSONArrayNotNull(t *testing.T) {
	srv := httptest.NewServer(New(store.New(), nil))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/stories")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	buf := make([]byte, 16)
	n, _ := resp.Body.Read(buf)
	got := string(buf[:n])
	if got != "[]\n" && got != "[]" {
		t.Fatalf("empty set should be [] not null, got %q", got)
	}
}

func TestStoriesBadParam(t *testing.T) {
	srv := httptest.NewServer(New(seeded(), nil))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/stories?min_score=abc")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad param should 400, got %d", resp.StatusCode)
	}
}

func TestTopDefaultAndN(t *testing.T) {
	_, out := doGet(t, New(seeded(), nil), "/stories/top")
	if len(out) != 3 {
		t.Fatalf("top default: got %d", len(out))
	}
	_, out = doGet(t, New(seeded(), nil), "/stories/top?n=2")
	if len(out) != 2 || out[0].ID != 1 {
		t.Fatalf("top n=2: got %v", out)
	}
}

func itoa(v int64) string { return strconv.FormatInt(v, 10) }
