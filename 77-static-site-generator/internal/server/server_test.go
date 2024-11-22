package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ccsg/internal/builder"
	"ccsg/internal/scaffold"
)

func newSite(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := scaffold.NewSite(root); err != nil {
		t.Fatal(err)
	}
	theme := "<html><head><title>{{ Title }}</title></head><body>{{ Content }}</body></html>\n"
	if err := os.WriteFile(filepath.Join(root, "themes", "default", "index.html"), []byte(theme), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "content", "index.md"), []byte("# Home\n\nfirst version\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := builder.Build(root); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestServesBuiltFileWithSnippet(t *testing.T) {
	root := newSite(t)
	ts := httptest.NewServer(New(root).Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/index.html")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	s := string(body)
	if !strings.Contains(s, "first version") {
		t.Errorf("served body missing content: %q", s)
	}
	if !strings.Contains(s, "/__livereload") {
		t.Errorf("live-reload snippet not injected: %q", s)
	}
}

func TestRootServesIndex(t *testing.T) {
	root := newSite(t)
	ts := httptest.NewServer(New(root).Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Home") {
		t.Errorf("root did not serve index: %q", string(body))
	}
}

func TestMissingFile404(t *testing.T) {
	root := newSite(t)
	ts := httptest.NewServer(New(root).Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/nope.html")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestSSEContentType(t *testing.T) {
	root := newSite(t)
	srv := New(root)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/__livereload", nil)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
}

func TestWatcherRebuilds(t *testing.T) {
	root := newSite(t)
	srv := New(root)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	sig := srv.Signature()

	// Edit the content file.
	if err := os.WriteFile(filepath.Join(root, "content", "index.md"), []byte("# Home\n\nsecond version\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Bounded poll: trigger a check until the served HTML updates.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		newSig, rebuilt, err := srv.CheckOnce(sig)
		if err != nil {
			t.Fatal(err)
		}
		sig = newSig
		if rebuilt {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	resp, err := http.Get(ts.URL + "/index.html")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "second version") {
		t.Errorf("served HTML did not update after rebuild: %q", string(body))
	}
}

func TestPathTraversalBlocked(t *testing.T) {
	root := newSite(t)
	// Secret file outside public/.
	if err := os.WriteFile(filepath.Join(root, "secret.txt"), []byte("top secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(New(root).Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/../secret.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "top secret") {
		t.Errorf("path traversal exposed file: %q", string(body))
	}
}
