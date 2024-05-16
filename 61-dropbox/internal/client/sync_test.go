package client

import (
	"bytes"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"dropbox/internal/hash"
	"dropbox/internal/server"
)

const testToken = "tok-alice"

func newTestServer(t *testing.T) (*Client, *server.Server) {
	t.Helper()
	srv := server.New(t.TempDir(), server.Config{
		Users: []server.User{{
			Name:         "alice",
			PasswordHash: server.HashPassword("pw"),
			Token:        testToken,
		}},
	})
	hs := httptest.NewServer(srv.Handler())
	t.Cleanup(hs.Close)
	return New(hs.URL, testToken, hs.Client()), srv
}

func newSyncer(t *testing.T, c *Client, now time.Time) (*Syncer, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".dropbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	return &Syncer{
		Root:   root,
		Ignore: []string{"*.tmp"},
		Client: c,
		Now:    func() time.Time { return now },
		Logf:   func(string, ...any) {},
	}, root
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

var fixed = time.Date(2026, 6, 26, 13, 5, 9, 0, time.UTC)

func TestUploadPath(t *testing.T) {
	t.Parallel()
	c, _ := newTestServer(t)
	s, root := newSyncer(t, c, fixed)
	writeFile(t, root, "docs/a.txt", "hello")
	writeFile(t, root, "skip.tmp", "ignored")

	res, err := s.SyncOnce()
	if err != nil {
		t.Fatal(err)
	}
	if res.Uploaded != 1 {
		t.Fatalf("uploaded = %d, want 1", res.Uploaded)
	}
	files, _ := c.List()
	if len(files) != 1 || files[0].Path != "docs/a.txt" {
		t.Fatalf("server files = %+v", files)
	}
	if files[0].Hash != hash.Bytes([]byte("hello")) || files[0].Version != 1 {
		t.Fatalf("server file meta wrong: %+v", files[0])
	}
	// Second sync is a no-op (converged, baseline advanced).
	res2, _ := s.SyncOnce()
	if len(res2.Actions) != 0 {
		t.Fatalf("second sync not idempotent: %+v", res2.Actions)
	}
}

func TestDownloadPath(t *testing.T) {
	t.Parallel()
	c, _ := newTestServer(t)
	s, root := newSyncer(t, c, fixed)
	body := []byte("remote content")
	if _, err := c.Upload("nested/dir/r.txt", 0, hash.Bytes(body), int64(len(body)), bytesReader(body)); err != nil {
		t.Fatal(err)
	}
	res, err := s.SyncOnce()
	if err != nil {
		t.Fatal(err)
	}
	if res.Downloaded != 1 {
		t.Fatalf("downloaded = %d, want 1", res.Downloaded)
	}
	if got := readFile(t, root, "nested/dir/r.txt"); got != "remote content" {
		t.Fatalf("downloaded content = %q", got)
	}
}

func TestModifyRoundTrip(t *testing.T) {
	t.Parallel()
	c, _ := newTestServer(t)
	s, root := newSyncer(t, c, fixed)
	writeFile(t, root, "f.txt", "v1")
	if _, err := s.SyncOnce(); err != nil {
		t.Fatal(err)
	}
	// Edit locally -> server version bumps.
	writeFile(t, root, "f.txt", "v2-local")
	if _, err := s.SyncOnce(); err != nil {
		t.Fatal(err)
	}
	files, _ := c.List()
	if files[0].Version != 2 || files[0].Hash != hash.Bytes([]byte("v2-local")) {
		t.Fatalf("after local edit: %+v", files[0])
	}
	// Edit on the server -> local updates.
	body := []byte("v3-remote")
	if _, err := c.Upload("f.txt", 2, hash.Bytes(body), int64(len(body)), bytesReader(body)); err != nil {
		t.Fatal(err)
	}
	res, _ := s.SyncOnce()
	if res.Downloaded != 1 {
		t.Fatalf("expected download of server edit, got %+v", res.Actions)
	}
	if got := readFile(t, root, "f.txt"); got != "v3-remote" {
		t.Fatalf("local content = %q", got)
	}
}

func TestDeletePropagation(t *testing.T) {
	t.Parallel()
	c, _ := newTestServer(t)
	s, root := newSyncer(t, c, fixed)
	writeFile(t, root, "keep.txt", "k")
	writeFile(t, root, "gone.txt", "g")
	if _, err := s.SyncOnce(); err != nil {
		t.Fatal(err)
	}
	// Delete locally -> removed on server.
	if err := os.Remove(filepath.Join(root, "gone.txt")); err != nil {
		t.Fatal(err)
	}
	res, _ := s.SyncOnce()
	if res.DeletedRemote != 1 {
		t.Fatalf("delete-remote = %d, want 1", res.DeletedRemote)
	}
	if files, _ := c.List(); len(files) != 1 || files[0].Path != "keep.txt" {
		t.Fatalf("server files after delete = %+v", files)
	}
	// Delete on server -> removed locally.
	rfiles, _ := c.List()
	if err := c.Delete("keep.txt", rfiles[0].Version); err != nil {
		t.Fatal(err)
	}
	res2, _ := s.SyncOnce()
	if res2.DeletedLocal != 1 {
		t.Fatalf("delete-local = %d, want 1", res2.DeletedLocal)
	}
	if _, err := os.Stat(filepath.Join(root, "keep.txt")); !os.IsNotExist(err) {
		t.Fatalf("keep.txt should be gone locally")
	}
}

func TestRenamePropagation(t *testing.T) {
	t.Parallel()
	c, _ := newTestServer(t)
	s, root := newSyncer(t, c, fixed)
	writeFile(t, root, "old.txt", "content")
	if _, err := s.SyncOnce(); err != nil {
		t.Fatal(err)
	}
	// Rename = delete old + create new.
	if err := os.Rename(filepath.Join(root, "old.txt"), filepath.Join(root, "new.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SyncOnce(); err != nil {
		t.Fatal(err)
	}
	files, _ := c.List()
	if len(files) != 1 || files[0].Path != "new.txt" {
		t.Fatalf("after rename server = %+v", files)
	}
}

func TestConflict(t *testing.T) {
	t.Parallel()
	c, _ := newTestServer(t)
	s, root := newSyncer(t, c, fixed)
	writeFile(t, root, "a.txt", "base")
	if _, err := s.SyncOnce(); err != nil {
		t.Fatal(err)
	}
	// Diverge: local and remote both change a.txt to different content.
	writeFile(t, root, "a.txt", "local-edit")
	body := []byte("remote-edit")
	files, _ := c.List()
	if _, err := c.Upload("a.txt", files[0].Version, hash.Bytes(body), int64(len(body)), bytesReader(body)); err != nil {
		t.Fatal(err)
	}

	res, err := s.SyncOnce()
	if err != nil {
		t.Fatal(err)
	}
	if res.Conflicts != 1 {
		t.Fatalf("conflicts = %d, want 1", res.Conflicts)
	}
	// Canonical path holds the server copy.
	if got := readFile(t, root, "a.txt"); got != "remote-edit" {
		t.Fatalf("canonical a.txt = %q, want remote-edit", got)
	}
	// Local edit preserved in a deterministic sidecar, both locally and on server.
	sidecar := "a (conflicted copy 2026-06-26 13-05-09).txt"
	if got := readFile(t, root, sidecar); got != "local-edit" {
		t.Fatalf("sidecar = %q, want local-edit", got)
	}
	rfiles, _ := c.List()
	var haveSidecar bool
	for _, f := range rfiles {
		if f.Path == sidecar {
			haveSidecar = true
		}
	}
	if !haveSidecar {
		t.Fatalf("sidecar not uploaded; server = %+v", rfiles)
	}
	// Converges afterwards.
	res2, _ := s.SyncOnce()
	if len(res2.Actions) != 0 {
		t.Fatalf("post-conflict sync not converged: %+v", res2.Actions)
	}
}

func TestAuthRequired(t *testing.T) {
	t.Parallel()
	_, srv := newTestServer(t)
	hs := httptest.NewServer(srv.Handler())
	t.Cleanup(hs.Close)
	bad := New(hs.URL, "wrong-token", hs.Client())
	if _, err := bad.List(); err != ErrAuth {
		t.Fatalf("want ErrAuth, got %v", err)
	}
}

// TestTwoClientConvergence is the FR-8 assertion: two clients against one
// account, after enough rounds, leave both roots byte-identical.
func TestTwoClientConvergence(t *testing.T) {
	t.Parallel()
	c, _ := newTestServer(t)
	s1, r1 := newSyncer(t, c, fixed)
	s2, r2 := newSyncer(t, New(c.base, c.token, c.hc), fixed)

	writeFile(t, r1, "from1.txt", "one")
	writeFile(t, r1, "shared/x.txt", "x")
	writeFile(t, r2, "from2.txt", "two")
	writeFile(t, r2, "shared/x.txt", "x") // identical content -> converges, no conflict

	for i := 0; i < 4; i++ {
		if _, err := s1.SyncOnce(); err != nil {
			t.Fatal(err)
		}
		if _, err := s2.SyncOnce(); err != nil {
			t.Fatal(err)
		}
	}

	t1 := readTree(t, r1)
	t2 := readTree(t, r2)
	if len(t1) != 3 {
		t.Fatalf("root1 tree = %v", t1)
	}
	if !treesEqual(t1, t2) {
		t.Fatalf("roots not byte-identical:\n r1=%v\n r2=%v", t1, t2)
	}
	// Server holds the same set.
	files, _ := c.List()
	if len(files) != 3 {
		t.Fatalf("server has %d files, want 3: %+v", len(files), files)
	}
}

// TestResumability simulates an interrupted run: the planner produces an upload
// but a transport error aborts it; the next sync converges with no corruption.
func TestResumability(t *testing.T) {
	t.Parallel()
	c, _ := newTestServer(t)
	s, root := newSyncer(t, c, fixed)
	writeFile(t, root, "a.txt", "data")

	// First pass: break the client so the upload fails.
	broken := New("http://127.0.0.1:1", c.token, c.hc) // unroutable
	s.Client = broken
	res, _ := s.SyncOnce()
	if res.Uploaded != 0 {
		t.Fatalf("upload should have failed, got %d", res.Uploaded)
	}
	// Baseline must NOT have advanced for the failed path.
	s.Client = c
	res2, err := s.SyncOnce()
	if err != nil {
		t.Fatal(err)
	}
	if res2.Uploaded != 1 {
		t.Fatalf("retry upload = %d, want 1", res2.Uploaded)
	}
	files, _ := c.List()
	if len(files) != 1 || files[0].Hash != hash.Bytes([]byte("data")) {
		t.Fatalf("server after retry = %+v", files)
	}
}

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }

func readTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if rel == ".dropbox/index.json" || rel == ".dropbox/config.json" {
			return nil
		}
		b, _ := os.ReadFile(p)
		out[rel] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func treesEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	ka := make([]string, 0, len(a))
	for k := range a {
		ka = append(ka, k)
	}
	sort.Strings(ka)
	for _, k := range ka {
		if a[k] != b[k] {
			return false
		}
	}
	return true
}
