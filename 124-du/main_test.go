package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"du/internal/walk"
)

func writeFile(t *testing.T, path string, n int) {
	t.Helper()
	if err := os.WriteFile(path, make([]byte, n), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// tree builds the canonical fixture and returns the root path.
func tree(t *testing.T) string {
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

// base uses exact-byte (apparent) sizing so output sizes are deterministic.
func base() walk.Options {
	return walk.Options{Apparent: true, MaxDepth: -1, Unit: walk.UnitCustom, BlockSize: 1}
}

// run executes Run and returns stdout, stderr, and the exit code.
func run(opts walk.Options, paths ...string) (string, string, int) {
	var out, errBuf bytes.Buffer
	code := Run(opts, paths, &out, &errBuf)
	return out.String(), errBuf.String(), code
}

// lineFor returns the size column for the line whose path ends with suffix.
func lineFor(t *testing.T, out, suffix string) (int64, bool) {
	t.Helper()
	for _, ln := range strings.Split(strings.TrimSpace(out), "\n") {
		parts := strings.SplitN(ln, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.HasSuffix(parts[1], suffix) {
			n, err := strconv.ParseInt(parts[0], 10, 64)
			if err != nil {
				t.Fatalf("bad size column %q in line %q", parts[0], ln)
			}
			return n, true
		}
	}
	return 0, false
}

func countLines(out string) int {
	out = strings.TrimSpace(out)
	if out == "" {
		return 0
	}
	return len(strings.Split(out, "\n"))
}

func TestSummarize(t *testing.T) {
	root := tree(t)
	o := base()
	o.Summarize = true
	out, _, code := run(o, root)

	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if n := countLines(out); n != 1 {
		t.Fatalf("summary should be one line, got %d:\n%s", n, out)
	}
	if sz, _ := lineFor(t, out, filepath.Base(root)); sz != 6000 {
		t.Errorf("summary size = %d, want 6000", sz)
	}
	if strings.Contains(out, "sub") {
		t.Errorf("summary should not list sub:\n%s", out)
	}
}

func TestMaxDepth(t *testing.T) {
	root := tree(t)

	// -d 0 == root only.
	o := base()
	o.MaxDepth = 0
	out, _, _ := run(o, root)
	if n := countLines(out); n != 1 {
		t.Errorf("-d 0 should print 1 line, got %d:\n%s", n, out)
	}

	// -d 1 == root + sub, no leaf files; sub still totals 5000.
	o = base()
	o.MaxDepth = 1
	out, _, _ = run(o, root)
	if sz, ok := lineFor(t, out, string(os.PathSeparator)+"sub"); !ok || sz != 5000 {
		t.Errorf("sub line = %d (present=%v), want 5000", sz, ok)
	}
	if _, ok := lineFor(t, out, "b.bin"); ok {
		t.Error("-d 1 should not print leaf files")
	}
	if sz, _ := lineFor(t, out, filepath.Base(root)); sz != 6000 {
		t.Errorf("root still = %d, want 6000", sz)
	}
}

func TestAllFiles(t *testing.T) {
	root := tree(t)
	o := base()
	o.All = true
	out, _, _ := run(o, root)

	for name, want := range map[string]int64{"a.bin": 1000, "b.bin": 2000, "c.bin": 3000} {
		if sz, ok := lineFor(t, out, name); !ok || sz != want {
			t.Errorf("%s = %d (present=%v), want %d", name, sz, ok, want)
		}
	}
	if sz, _ := lineFor(t, out, string(os.PathSeparator)+"sub"); sz != 5000 {
		t.Errorf("sub still = %d, want 5000", sz)
	}
}

func TestGrandTotal(t *testing.T) {
	r1, r2 := tree(t), tree(t)
	o := base()
	o.Summarize = true
	o.Total = true
	out, _, _ := run(o, r1, r2)

	if sz, ok := lineFor(t, out, "total"); !ok || sz != 12000 {
		t.Errorf("total line = %d (present=%v), want 12000", sz, ok)
	}
}

func TestThreshold(t *testing.T) {
	root := tree(t)
	o := base()
	o.All = true
	o.Threshold = 2500
	out, _, _ := run(o, root)

	// >= 2500: root (6000), sub (5000), c.bin (3000).
	if _, ok := lineFor(t, out, "c.bin"); !ok {
		t.Error("c.bin (3000) should pass threshold 2500")
	}
	if _, ok := lineFor(t, out, "b.bin"); ok {
		t.Error("b.bin (2000) should be filtered by threshold 2500")
	}
	if _, ok := lineFor(t, out, "a.bin"); ok {
		t.Error("a.bin (1000) should be filtered by threshold 2500")
	}
	if sz, _ := lineFor(t, out, string(os.PathSeparator)+"sub"); sz != 5000 {
		t.Errorf("sub still accumulates to %d, want 5000 (threshold filters output only)", sz)
	}
}

func TestExcludeCLI(t *testing.T) {
	root := tree(t)
	o := base()
	o.All = true
	o.Excludes = []string{"*.bin"}
	out, _, _ := run(o, root)

	if strings.Contains(out, ".bin") {
		t.Errorf("excluded .bin files should be absent:\n%s", out)
	}
	if sz, _ := lineFor(t, out, filepath.Base(root)); sz != 0 {
		t.Errorf("root with all .bin excluded = %d, want 0", sz)
	}
}

func TestHumanReadableEndToEnd(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "big.bin"), 1536) // 1.5K apparent
	o := walk.Options{Apparent: true, MaxDepth: -1, Unit: walk.UnitHuman, All: true}
	out, _, _ := run(o, dir)

	if !strings.Contains(out, "1.5K") {
		t.Errorf("-h output should contain 1.5K:\n%s", out)
	}
}

func TestExitCodeOnMissingPath(t *testing.T) {
	_, errBuf, code := run(base(), filepath.Join(t.TempDir(), "nope"))
	if code != 1 {
		t.Errorf("exit code for missing path = %d, want 1", code)
	}
	if errBuf == "" {
		t.Error("expected stderr message for missing path")
	}
}

func TestParseArgs(t *testing.T) {
	o, paths, err := parseArgs([]string{"-sh", "--exclude=*.tmp", "-d", "2", "x", "y"})
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if !o.Summarize || o.Unit != walk.UnitHuman {
		t.Errorf("bundled -sh not parsed: %+v", o)
	}
	if o.MaxDepth != 2 {
		t.Errorf("MaxDepth = %d, want 2", o.MaxDepth)
	}
	if len(o.Excludes) != 1 || o.Excludes[0] != "*.tmp" {
		t.Errorf("excludes = %v", o.Excludes)
	}
	if strings.Join(paths, ",") != "x,y" {
		t.Errorf("paths = %v", paths)
	}

	if _, _, err := parseArgs([]string{"-a", "-s"}); err == nil {
		t.Error("expected error for -a with -s")
	}
	if _, _, err := parseArgs([]string{"-t", "1K"}); err != nil {
		t.Errorf("-t 1K should parse: %v", err)
	}
}
