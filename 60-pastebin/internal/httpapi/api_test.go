package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"pastebin/internal/clock"
	"pastebin/internal/httpapi"
	"pastebin/internal/paste"
	"pastebin/internal/store"
)

type testEnv struct {
	handler http.Handler
	clk     *clock.Fake
	store   store.Store
}

func newTestEnv(t *testing.T, maxBytes int64) testEnv {
	t.Helper()
	st := store.NewMemStore()
	clk := clock.NewFake(time.Date(2026, 6, 26, 14, 0, 0, 0, time.UTC))
	svc := paste.NewService(st, clk, paste.Config{MaxBytes: maxBytes})
	srv, err := httpapi.NewServer(httpapi.Options{
		Service:  svc,
		Store:    st,
		BaseURL:  "http://example.test",
		MaxBytes: maxBytes,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return testEnv{handler: srv.Handler(), clk: clk, store: st}
}

func (e testEnv) do(t *testing.T, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

func jsonReq(t *testing.T, method, path, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// createViaAPI is a helper returning the decoded create response.
func (e testEnv) createViaAPI(t *testing.T, body string) map[string]any {
	t.Helper()
	rec := e.do(t, jsonReq(t, "POST", "/api/pastes", body))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: got %d, body %s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	return out
}

func TestAPICreateHappyPath(t *testing.T) {
	e := newTestEnv(t, 1<<20)
	out := e.createViaAPI(t, `{"content":"hello world","language":"go","expires_in":"1h"}`)

	id, _ := out["id"].(string)
	if id == "" {
		t.Fatal("missing id")
	}
	if out["url"] != "http://example.test/"+id {
		t.Fatalf("url: got %v", out["url"])
	}
	if out["raw_url"] != "http://example.test/"+id+"/raw" {
		t.Fatalf("raw_url: got %v", out["raw_url"])
	}
	if dk, _ := out["delete_key"].(string); dk == "" {
		t.Fatal("missing delete_key")
	}
	if out["expires_at"] == nil {
		t.Fatal("expected expires_at to be set")
	}
}

func TestAPIGet(t *testing.T) {
	e := newTestEnv(t, 1<<20)
	out := e.createViaAPI(t, `{"content":"some content"}`)
	id := out["id"].(string)

	rec := e.do(t, httptest.NewRequest("GET", "/api/pastes/"+id, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get: got %d", rec.Code)
	}
	var got map[string]any
	json.Unmarshal(rec.Body.Bytes(), &got)
	if got["content"] != "some content" {
		t.Fatalf("content: got %v", got["content"])
	}
}

func TestAPIGetUnknown404(t *testing.T) {
	e := newTestEnv(t, 1<<20)
	rec := e.do(t, httptest.NewRequest("GET", "/api/pastes/doesnotexist", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", rec.Code)
	}
}

func TestAPIGetExpired410(t *testing.T) {
	e := newTestEnv(t, 1<<20)
	out := e.createViaAPI(t, `{"content":"temp","expires_in":"1h"}`)
	id := out["id"].(string)

	e.clk.Advance(2 * time.Hour)
	rec := e.do(t, httptest.NewRequest("GET", "/api/pastes/"+id, nil))
	if rec.Code != http.StatusGone {
		t.Fatalf("got %d, want 410", rec.Code)
	}
}

func TestRawView(t *testing.T) {
	e := newTestEnv(t, 1<<20)
	const content = "line1\nline2\n<tag>"
	out := e.createViaAPI(t, `{"content":`+jsonString(content)+`}`)
	id := out["id"].(string)

	for _, path := range []string{"/" + id + "/raw", "/api/pastes/" + id + "/raw"} {
		rec := e.do(t, httptest.NewRequest("GET", path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: got %d", path, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
			t.Fatalf("%s: content-type %q", path, ct)
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("%s: missing nosniff", path)
		}
		if rec.Body.String() != content {
			t.Fatalf("%s: body %q, want %q", path, rec.Body.String(), content)
		}
	}
}

func TestRawUnknown404(t *testing.T) {
	e := newTestEnv(t, 1<<20)
	rec := e.do(t, httptest.NewRequest("GET", "/nope/raw", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", rec.Code)
	}
}

func TestAPICreateOverSize(t *testing.T) {
	e := newTestEnv(t, 10) // 10-byte max
	body := `{"content":` + jsonString(strings.Repeat("x", 50)) + `}`
	rec := e.do(t, jsonReq(t, "POST", "/api/pastes", body))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("got %d, want 413", rec.Code)
	}
}

func TestAPICreateEmpty400(t *testing.T) {
	e := newTestEnv(t, 1<<20)
	rec := e.do(t, jsonReq(t, "POST", "/api/pastes", `{"content":""}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", rec.Code)
	}
}

func TestAPIMalformedJSON400(t *testing.T) {
	e := newTestEnv(t, 1<<20)
	rec := e.do(t, jsonReq(t, "POST", "/api/pastes", `{"content": `))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", rec.Code)
	}
}

func TestAPIDelete(t *testing.T) {
	e := newTestEnv(t, 1<<20)
	out := e.createViaAPI(t, `{"content":"deleteme"}`)
	id := out["id"].(string)
	key := out["delete_key"].(string)

	// Wrong key -> 403.
	req := httptest.NewRequest("DELETE", "/api/pastes/"+id, nil)
	req.Header.Set("X-Delete-Key", "wrong")
	if rec := e.do(t, req); rec.Code != http.StatusForbidden {
		t.Fatalf("wrong key: got %d, want 403", rec.Code)
	}

	// Correct key -> 204.
	req = httptest.NewRequest("DELETE", "/api/pastes/"+id, nil)
	req.Header.Set("X-Delete-Key", key)
	if rec := e.do(t, req); rec.Code != http.StatusNoContent {
		t.Fatalf("correct key: got %d, want 204", rec.Code)
	}

	// Now gone -> 404.
	if rec := e.do(t, httptest.NewRequest("GET", "/api/pastes/"+id, nil)); rec.Code != http.StatusNotFound {
		t.Fatalf("after delete: got %d, want 404", rec.Code)
	}

	// Delete unknown -> 404.
	req = httptest.NewRequest("DELETE", "/api/pastes/unknownid", nil)
	req.Header.Set("X-Delete-Key", "whatever")
	if rec := e.do(t, req); rec.Code != http.StatusNotFound {
		t.Fatalf("delete unknown: got %d, want 404", rec.Code)
	}
}

func TestHealthz(t *testing.T) {
	e := newTestEnv(t, 1<<20)
	rec := e.do(t, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ok") {
		t.Fatalf("body: %s", rec.Body.String())
	}
}

func TestWebCreateRedirect(t *testing.T) {
	e := newTestEnv(t, 1<<20)
	form := url.Values{}
	form.Set("content", "web paste")
	form.Set("expires_in", "never")
	form.Set("language", "go")

	req := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := e.do(t, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("got %d, want 303", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/") || len(strings.TrimPrefix(loc, "/")) == 0 {
		t.Fatalf("Location %q is not a paste path", loc)
	}
	// The redirect target should resolve to a real paste.
	rec2 := e.do(t, httptest.NewRequest("GET", loc, nil))
	if rec2.Code != http.StatusOK {
		t.Fatalf("following redirect: got %d", rec2.Code)
	}
}

func TestWebViewEscapesXSS(t *testing.T) {
	e := newTestEnv(t, 1<<20)
	payload := `<script>alert('xss')</script>`
	out := e.createViaAPI(t, `{"content":`+jsonString(payload)+`}`)
	id := out["id"].(string)

	rec := e.do(t, httptest.NewRequest("GET", "/"+id, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("view: got %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "<script>alert") {
		t.Fatal("raw <script> leaked into HTML (stored XSS)")
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Fatal("expected escaped script tag in HTML")
	}
}

func TestWebViewNotFound(t *testing.T) {
	e := newTestEnv(t, 1<<20)
	rec := e.do(t, httptest.NewRequest("GET", "/missingpaste", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", rec.Code)
	}
}

func TestWebDeleteFlow(t *testing.T) {
	e := newTestEnv(t, 1<<20)
	out := e.createViaAPI(t, `{"content":"web delete"}`)
	id := out["id"].(string)
	key := out["delete_key"].(string)

	form := url.Values{}
	form.Set("delete_key", key)
	req := httptest.NewRequest("POST", "/"+id+"/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := e.do(t, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "deleted") {
		t.Fatalf("expected deleted page, got: %s", rec.Body.String())
	}

	// Idempotent: deleting again still renders the deleted page.
	req2 := httptest.NewRequest("POST", "/"+id+"/delete", strings.NewReader(form.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if rec2 := e.do(t, req2); rec2.Code != http.StatusOK {
		t.Fatalf("idempotent delete: got %d", rec2.Code)
	}

	// Store no longer has it.
	if _, err := e.store.Get(context.Background(), id); err == nil {
		t.Fatal("expected paste to be gone from store")
	}
}

// jsonString quotes s as a JSON string literal.
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
