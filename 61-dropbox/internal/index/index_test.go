package index

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"dropbox/internal/hash"
)

func write(t *testing.T, root, rel, content string) string {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestScanNestedAndIgnore(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "a.txt", "a")
	write(t, root, "dir/b.txt", "b")
	write(t, root, "dir/sub/c.txt", "c")
	write(t, root, "scratch.tmp", "tmp")
	write(t, root, ".dropbox/index.json", "{}")
	write(t, root, "node_modules/x.js", "x")

	idx, err := Scan(root, []string{"*.tmp", "node_modules/"}, nil, hash.File)
	if err != nil {
		t.Fatal(err)
	}
	got := idx.Paths()
	want := []string{"a.txt", "dir/b.txt", "dir/sub/c.txt"}
	if len(got) != len(want) {
		t.Fatalf("paths = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("paths = %v, want %v", got, want)
		}
	}
	if idx["a.txt"].Hash != hash.Bytes([]byte("a")) {
		t.Fatalf("a.txt hash mismatch")
	}
}

func TestScanFastPathSkipsRehash(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "a.txt", "hello")
	write(t, root, "b.txt", "world")

	var calls int64
	counting := func(p string) (string, error) {
		atomic.AddInt64(&calls, 1)
		return hash.File(p)
	}

	first, err := Scan(root, nil, nil, counting)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("first scan hashed %d files, want 2", calls)
	}

	// Re-scan with the previous index: nothing changed -> no re-hash.
	atomic.StoreInt64(&calls, 0)
	if _, err := Scan(root, nil, first, counting); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("re-scan hashed %d files, want 0 (fast path)", calls)
	}

	// Change mtime of a.txt -> it must be re-hashed.
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(filepath.Join(root, "a.txt"), future, future); err != nil {
		t.Fatal(err)
	}
	atomic.StoreInt64(&calls, 0)
	if _, err := Scan(root, nil, first, counting); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("after mtime change hashed %d files, want 1", calls)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	write(t, root, "a.txt", "a")
	write(t, root, "dir/b.txt", "b")
	idx, err := Scan(root, nil, nil, hash.File)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, ".dropbox", "index.json")
	if err := idx.Save(p); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(idx) {
		t.Fatalf("loaded %d entries, want %d", len(got), len(idx))
	}
	for k, v := range idx {
		gv, ok := got[k]
		if !ok || gv.Hash != v.Hash || gv.Size != v.Size || !gv.ModTime.Equal(v.ModTime) {
			t.Fatalf("entry %s mismatch: %+v vs %+v", k, gv, v)
		}
	}
}

func TestLoadMissingIsEmpty(t *testing.T) {
	t.Parallel()
	idx, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(idx) != 0 {
		t.Fatalf("want empty, got %d", len(idx))
	}
}
