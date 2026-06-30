//go:build !windows

package walk

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHardlinkDedupUnix verifies that a hardlink to an already-counted inode
// contributes zero, so the directory total matches a tree with no link.
func TestHardlinkDedupUnix(t *testing.T) {
	dir := t.TempDir()
	orig := filepath.Join(dir, "b.bin")
	writeFile(t, orig, 2000)

	// Baseline: just the single file.
	w := New(apparentOpts(), io.Discard)
	_, baseline := w.Walk(dir)
	if baseline != 2000 {
		t.Fatalf("baseline = %d, want 2000", baseline)
	}

	// Add a hardlink to the same inode.
	if err := os.Link(orig, filepath.Join(dir, "b2.bin")); err != nil {
		t.Skipf("hardlinks unsupported here: %v", err)
	}
	w2 := New(apparentOpts(), io.Discard)
	_, withLink := w2.Walk(dir)
	if withLink != 2000 {
		t.Errorf("total with hardlink = %d, want 2000 (counted once)", withLink)
	}
}

// TestPermissionErrorUnix chmod 000s a subdirectory and asserts the run reports
// the error but still reports the readable part of the tree.
func TestPermissionErrorUnix(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses permission bits")
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.bin"), 1000)
	locked := filepath.Join(root, "locked")
	if err := os.Mkdir(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(locked, "secret.bin"), 4000)
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	var sb strings.Builder
	w := New(apparentOpts(), &sb)
	entries, _ := w.Walk(root)

	if !w.HadError() {
		t.Error("expected HadError() == true for unreadable subdir")
	}
	if sb.Len() == 0 {
		t.Error("expected an error message on stderr")
	}
	if _, ok := find(entries, "a.bin"); !ok {
		t.Error("readable a.bin should still be reported despite permission error")
	}
}
