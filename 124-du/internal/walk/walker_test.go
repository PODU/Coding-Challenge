package walk

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile creates a file of exactly n bytes.
func writeFile(t *testing.T, path string, n int) {
	t.Helper()
	if err := os.WriteFile(path, make([]byte, n), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// buildTree builds the canonical fixture from the design doc:
//
//	root/
//	  a.bin   (1000 bytes)
//	  sub/
//	    b.bin (2000 bytes)
//	    c.bin (3000 bytes)
func buildTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.bin"), 1000)
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(sub, "b.bin"), 2000)
	writeFile(t, filepath.Join(sub, "c.bin"), 3000)
	return root
}

// find returns the entry whose path ends with suffix.
func find(entries []Entry, suffix string) (Entry, bool) {
	for _, e := range entries {
		if strings.HasSuffix(e.Path, suffix) {
			return e, true
		}
	}
	return Entry{}, false
}

// apparentOpts uses logical sizes so totals are deterministic across platforms
// (directories contribute 0 in apparent mode).
func apparentOpts() Options {
	return Options{Apparent: true, MaxDepth: -1, Unit: UnitCustom, BlockSize: 1}
}

func TestAccumulationBottomUp(t *testing.T) {
	root := buildTree(t)
	w := New(apparentOpts(), io.Discard)
	entries, total := w.Walk(root)

	if total != 6000 {
		t.Errorf("root total = %d, want 6000", total)
	}
	sub, ok := find(entries, filepath.Join("root", "sub"))
	if !ok {
		// path won't literally contain "root"; match on trailing "sub".
		sub, ok = find(entries, string(os.PathSeparator)+"sub")
	}
	if !ok {
		t.Fatalf("sub entry not found in %+v", entries)
	}
	if sub.Size != 5000 {
		t.Errorf("sub total = %d, want 5000", sub.Size)
	}
	rootEntry, ok := find(entries, filepath.Base(root))
	if !ok || rootEntry.Depth != 0 {
		t.Fatalf("root entry missing or wrong depth: %+v", rootEntry)
	}
	if rootEntry.Size != 6000 {
		t.Errorf("root entry size = %d, want 6000", rootEntry.Size)
	}
	// Parent must appear after its children (deepest-first ordering).
	if entries[len(entries)-1].Depth != 0 {
		t.Errorf("last entry should be the root (depth 0), got depth %d", entries[len(entries)-1].Depth)
	}
}

func TestExcludeByDirName(t *testing.T) {
	root := buildTree(t)
	opts := apparentOpts()
	opts.Excludes = []string{"sub"}
	w := New(opts, io.Discard)
	entries, total := w.Walk(root)

	if total != 1000 {
		t.Errorf("total with sub excluded = %d, want 1000 (a.bin only)", total)
	}
	if _, ok := find(entries, "b.bin"); ok {
		t.Error("b.bin should be absent when sub is excluded")
	}
}

func TestExcludeByGlob(t *testing.T) {
	root := buildTree(t)
	opts := apparentOpts()
	opts.All = true
	opts.Excludes = []string{"*.bin"}
	w := New(opts, io.Discard)
	entries, total := w.Walk(root)

	if total != 0 {
		t.Errorf("total with *.bin excluded = %d, want 0", total)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Path, ".bin") {
			t.Errorf("excluded file present: %s", e.Path)
		}
	}
}

func TestUnreadablePathSetsError(t *testing.T) {
	root := buildTree(t)
	missing := filepath.Join(root, "does-not-exist")

	var sb strings.Builder
	w := New(apparentOpts(), &sb)
	_, total := w.Walk(missing)

	if !w.HadError() {
		t.Error("HadError() = false, want true for a missing path")
	}
	if total != 0 {
		t.Errorf("total for missing path = %d, want 0", total)
	}
	if sb.Len() == 0 {
		t.Error("expected an error message on stderr")
	}

	// A readable sibling is still fully reported despite the earlier failure.
	_, good := w.Walk(root)
	if good != 6000 {
		t.Errorf("readable root after error = %d, want 6000", good)
	}
}

func TestAllFilesBuffered(t *testing.T) {
	root := buildTree(t)
	opts := apparentOpts()
	opts.All = true
	w := New(opts, io.Discard)
	entries, _ := w.Walk(root)

	for _, name := range []string{"a.bin", "b.bin", "c.bin"} {
		if _, ok := find(entries, name); !ok {
			t.Errorf("expected %s in -a output", name)
		}
	}
}
