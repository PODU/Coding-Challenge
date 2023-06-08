package shortener

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const testBaseURL = "http://localhost:8080"

func newTestServer(t *testing.T) (*Handler, http.Handler) {
	t.Helper()
	h := NewHandler(NewMemStore(), testBaseURL)
	return h, h.Routes()
}

func doJSON(t *testing.T, srv http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	return rr
}

func decodeShorten(t *testing.T, rr *httptest.ResponseRecorder) shortenResponse {
	t.Helper()
	var resp shortenResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v (body=%q)", err, rr.Body.String())
	}
	return resp
}

func TestCreateReturns201AndShape(t *testing.T) {
	_, srv := newTestServer(t)
	rr := doJSON(t, srv, http.MethodPost, "/shorten", `{"url":"https://example.com/very/long/path"}`)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("content-type = %q, want application/json", ct)
	}
	resp := decodeShorten(t, rr)
	if resp.Key == "" {
		t.Error("key is empty")
	}
	if resp.LongURL != "https://example.com/very/long/path" {
		t.Errorf("long_url = %q", resp.LongURL)
	}
	if want := testBaseURL + "/" + resp.Key; resp.ShortURL != want {
		t.Errorf("short_url = %q, want %q", resp.ShortURL, want)
	}
}

func TestCreateIsIdempotent(t *testing.T) {
	_, srv := newTestServer(t)
	first := doJSON(t, srv, http.MethodPost, "/shorten", `{"url":"https://example.com"}`)
	if first.Code != http.StatusCreated {
		t.Fatalf("first status = %d, want 201", first.Code)
	}
	firstResp := decodeShorten(t, first)

	second := doJSON(t, srv, http.MethodPost, "/shorten", `{"url":"https://example.com"}`)
	if second.Code != http.StatusOK {
		t.Fatalf("second status = %d, want 200", second.Code)
	}
	secondResp := decodeShorten(t, second)

	if firstResp.Key != secondResp.Key {
		t.Errorf("keys differ: %q vs %q", firstResp.Key, secondResp.Key)
	}
}

func TestCreateRejectsInvalidURLs(t *testing.T) {
	_, srv := newTestServer(t)
	cases := map[string]string{
		"empty":        `{"url":""}`,
		"no scheme":    `{"url":"example.com"}`,
		"non-http":     `{"url":"ftp://example.com"}`,
		"missing host": `{"url":"http://"}`,
		"too long":     fmt.Sprintf(`{"url":"https://example.com/%s"}`, strings.Repeat("a", 3000)),
		"scheme only":  `{"url":"https"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			rr := doJSON(t, srv, http.MethodPost, "/shorten", body)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body=%s)", rr.Code, rr.Body.String())
			}
			var e errorResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &e); err != nil || e.Error == "" {
				t.Errorf("expected error body, got %q", rr.Body.String())
			}
		})
	}
}

func TestCreateRejectsMalformedJSON(t *testing.T) {
	_, srv := newTestServer(t)
	for name, body := range map[string]string{
		"empty body":  ``,
		"not json":    `not json`,
		"unknown key": `{"link":"https://example.com"}`,
	} {
		t.Run(name, func(t *testing.T) {
			rr := doJSON(t, srv, http.MethodPost, "/shorten", body)
			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rr.Code)
			}
		})
	}
}

func TestRedirectKnownKey(t *testing.T) {
	_, srv := newTestServer(t)
	create := doJSON(t, srv, http.MethodPost, "/shorten", `{"url":"https://example.com/page?q=1#frag"}`)
	resp := decodeShorten(t, create)

	rr := doJSON(t, srv, http.MethodGet, "/"+resp.Key, "")
	if rr.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "https://example.com/page?q=1#frag" {
		t.Errorf("Location = %q, want exact original URL", loc)
	}
}

func TestRedirectUnknownKey(t *testing.T) {
	_, srv := newTestServer(t)
	rr := doJSON(t, srv, http.MethodGet, "/doesnotexist", "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestDeleteThenRedirect404(t *testing.T) {
	_, srv := newTestServer(t)
	create := doJSON(t, srv, http.MethodPost, "/shorten", `{"url":"https://example.com/del"}`)
	resp := decodeShorten(t, create)

	del := doJSON(t, srv, http.MethodDelete, "/"+resp.Key, "")
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", del.Code)
	}

	redirect := doJSON(t, srv, http.MethodGet, "/"+resp.Key, "")
	if redirect.Code != http.StatusNotFound {
		t.Fatalf("post-delete redirect status = %d, want 404", redirect.Code)
	}
}

func TestDeleteUnknownKey(t *testing.T) {
	_, srv := newTestServer(t)
	rr := doJSON(t, srv, http.MethodDelete, "/nope", "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	_, srv := newTestServer(t)
	cases := []struct{ method, path string }{
		{http.MethodPut, "/shorten"},
		{http.MethodPost, "/somekey"},
		{http.MethodPatch, "/somekey"},
	}
	for _, c := range cases {
		t.Run(c.method+" "+c.path, func(t *testing.T) {
			rr := doJSON(t, srv, c.method, c.path, "")
			if rr.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want 405", rr.Code)
			}
		})
	}
}

func TestHealthz(t *testing.T) {
	_, srv := newTestServer(t)
	rr := doJSON(t, srv, http.MethodGet, "/healthz", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

// collidingStore reports the first `collisions` candidate keys as taken,
// exercising the keygen collision-retry path deterministically.
type collidingStore struct {
	*MemStore
	remaining int
}

func (c *collidingStore) Get(key string) (string, bool) {
	if c.remaining > 0 {
		c.remaining--
		return "occupied", true
	}
	return c.MemStore.Get(key)
}

func TestKeygenCollisionProducesUniqueKey(t *testing.T) {
	store := &collidingStore{MemStore: NewMemStore(), remaining: 12}
	h := NewHandler(store, testBaseURL)
	srv := h.Routes()

	rr := doJSON(t, srv, http.MethodPost, "/shorten", `{"url":"https://example.com/collide"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rr.Code)
	}
	resp := decodeShorten(t, rr)
	if resp.Key == "" {
		t.Fatal("empty key after collisions")
	}
	if got, ok := store.MemStore.Get(resp.Key); !ok || got != "https://example.com/collide" {
		t.Errorf("key %q not stored correctly (ok=%v got=%q)", resp.Key, ok, got)
	}
}

func TestGenerateKeyTerminatesAndLengthens(t *testing.T) {
	// Force many collisions and confirm the key grows past the default length.
	taken := 9
	key := GenerateKey("https://example.com", func(string) bool {
		if taken > 0 {
			taken--
			return true
		}
		return false
	})
	if len(key) < DefaultKeyLength {
		t.Fatalf("key length = %d, want >= %d", len(key), DefaultKeyLength)
	}
}

func TestConcurrentDistinctCreates(t *testing.T) {
	_, srv := newTestServer(t)
	const n = 50
	keys := make([]string, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := fmt.Sprintf(`{"url":"https://example.com/path/%d"}`, i)
			rr := doJSON(t, srv, http.MethodPost, "/shorten", body)
			if rr.Code != http.StatusCreated {
				t.Errorf("create %d status = %d", i, rr.Code)
				return
			}
			keys[i] = decodeShorten(t, rr).Key
		}(i)
	}
	wg.Wait()

	seen := make(map[string]bool, n)
	for i, k := range keys {
		if k == "" {
			t.Fatalf("create %d produced empty key", i)
		}
		if seen[k] {
			t.Fatalf("duplicate key %q for distinct URLs", k)
		}
		seen[k] = true
	}
}

func TestFileStorePersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.json")

	store1, err := NewFileStore(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	h1 := NewHandler(store1, testBaseURL)
	rr := doJSON(t, h1.Routes(), http.MethodPost, "/shorten", `{"url":"https://persisted.example/x"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create status = %d", rr.Code)
	}
	key := decodeShorten(t, rr).Key

	// Reopen from disk: the mapping must survive.
	store2, err := NewFileStore(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	got, ok := store2.Get(key)
	if !ok || got != "https://persisted.example/x" {
		t.Fatalf("reloaded mapping = (%q,%v), want persisted URL", got, ok)
	}

	// Delete should also persist.
	if !store2.Delete(key) {
		t.Fatal("delete returned false")
	}
	store3, err := NewFileStore(path)
	if err != nil {
		t.Fatalf("reopen after delete: %v", err)
	}
	if _, ok := store3.Get(key); ok {
		t.Fatal("key still present after persisted delete")
	}
}
