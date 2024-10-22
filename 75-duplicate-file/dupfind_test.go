package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// writeFile writes content to a file under dir, creating parent dirs.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// basicTree builds the fixture described in the design's test plan and returns
// the root directory plus the two expected-duplicate paths.
func basicTree(t *testing.T) (root, aPath, aCopyPath string) {
	t.Helper()
	root = t.TempDir()
	dupContent := "hello world, this is duplicated content AAA"
	aPath = filepath.Join(root, "a.txt")
	aCopyPath = filepath.Join(root, "nested", "deep", "a_copy.txt")
	writeFile(t, aPath, dupContent)
	writeFile(t, aCopyPath, dupContent)
	// unique file (different size, never reported)
	writeFile(t, filepath.Join(root, "b.txt"), "totally unique content ZZZ different length")
	// same size, different content (survives size stage, split by hash stage)
	writeFile(t, filepath.Join(root, "c.txt"), "1234567890")
	writeFile(t, filepath.Join(root, "c2.txt"), "0987654321")
	return root, aPath, aCopyPath
}

func canon(t *testing.T, p string) string {
	t.Helper()
	a, err := filepath.Abs(p)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	return filepath.Clean(a)
}

func TestFindDuplicates_Basic(t *testing.T) {
	root, aPath, aCopyPath := basicTree(t)

	rep, stats, err := FindDuplicates([]string{root}, Options{MinSize: 1})
	if err != nil {
		t.Fatalf("FindDuplicates: %v", err)
	}
	if len(rep.Groups) != 1 {
		t.Fatalf("expected exactly 1 duplicate group, got %d: %+v", len(rep.Groups), rep.Groups)
	}
	g := rep.Groups[0]
	want := []string{canon(t, aPath), canon(t, aCopyPath)}
	sort.Strings(want)
	got := append([]string(nil), g.Paths...)
	sort.Strings(got)
	if !equalStrings(got, want) {
		t.Fatalf("group paths = %v, want %v", got, want)
	}
	if g.Wasted != g.Size {
		t.Fatalf("wasted = %d, want %d (one redundant copy)", g.Wasted, g.Size)
	}

	// c.txt and c2.txt share a size but differ; they must reach the size stage
	// but be dropped by the partial-hash stage before full hashing.
	if stats.SizeCandidates < 4 {
		t.Errorf("SizeCandidates = %d, want >= 4 (a-pair + c-pair)", stats.SizeCandidates)
	}
	if stats.PartialCandidates != 2 {
		t.Errorf("PartialCandidates = %d, want 2 (only the a-pair survives partial hash)", stats.PartialCandidates)
	}
	if stats.Confirmed != 2 {
		t.Errorf("Confirmed = %d, want 2", stats.Confirmed)
	}
}

func TestSizeSamePartialSplits(t *testing.T) {
	// Two files of equal size but different content must not be reported.
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "x.bin"), "AAAAAAAAAA")
	writeFile(t, filepath.Join(root, "y.bin"), "BBBBBBBBBB")

	rep, stats, err := FindDuplicates([]string{root}, Options{MinSize: 1})
	if err != nil {
		t.Fatalf("FindDuplicates: %v", err)
	}
	if len(rep.Groups) != 0 {
		t.Fatalf("expected no duplicates, got %d groups", len(rep.Groups))
	}
	if stats.SizeCandidates != 2 {
		t.Errorf("SizeCandidates = %d, want 2", stats.SizeCandidates)
	}
	if stats.PartialCandidates != 0 {
		t.Errorf("PartialCandidates = %d, want 0 (partial hash splits them)", stats.PartialCandidates)
	}
}

func TestEmptyFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "empty1.txt"), "")
	writeFile(t, filepath.Join(root, "empty2.txt"), "")
	writeFile(t, filepath.Join(root, "notempty.txt"), "x")

	// MinSize 0 so zero-byte files are included.
	rep, _, err := FindDuplicates([]string{root}, Options{MinSize: 0})
	if err != nil {
		t.Fatalf("FindDuplicates: %v", err)
	}
	if len(rep.Groups) != 1 {
		t.Fatalf("expected 1 group of empties, got %d: %+v", len(rep.Groups), rep.Groups)
	}
	g := rep.Groups[0]
	if g.Size != 0 || len(g.Paths) != 2 {
		t.Fatalf("empty group = size %d, %d paths; want size 0, 2 paths", g.Size, len(g.Paths))
	}

	// Default MinSize 1 must exclude empties entirely.
	rep2, _, err := FindDuplicates([]string{root}, Options{MinSize: 1})
	if err != nil {
		t.Fatalf("FindDuplicates: %v", err)
	}
	if len(rep2.Groups) != 0 {
		t.Fatalf("with MinSize 1, expected 0 groups, got %d", len(rep2.Groups))
	}
}

func TestMinSizeFilter(t *testing.T) {
	root := t.TempDir()
	small := "tiny"                    // 4 bytes
	large := strings.Repeat("L", 5000) // 5000 bytes
	writeFile(t, filepath.Join(root, "s1.txt"), small)
	writeFile(t, filepath.Join(root, "s2.txt"), small)
	writeFile(t, filepath.Join(root, "l1.txt"), large)
	writeFile(t, filepath.Join(root, "l2.txt"), large)

	rep, _, err := FindDuplicates([]string{root}, Options{MinSize: 1000})
	if err != nil {
		t.Fatalf("FindDuplicates: %v", err)
	}
	if len(rep.Groups) != 1 {
		t.Fatalf("expected only the large pair, got %d groups", len(rep.Groups))
	}
	if rep.Groups[0].Size != int64(len(large)) {
		t.Fatalf("group size = %d, want %d", rep.Groups[0].Size, len(large))
	}
}

func TestVerifyGroupSplitsOnContent(t *testing.T) {
	// Directly exercise the byte verifier: same size, force them into one group
	// and confirm it partitions different content into separate classes.
	root := t.TempDir()
	p1 := filepath.Join(root, "a")
	p2 := filepath.Join(root, "b")
	p3 := filepath.Join(root, "c")
	writeFile(t, p1, "AAAA")
	writeFile(t, p2, "AAAA")
	writeFile(t, p3, "BBBB")
	group := []FileInfo{
		{Path: p1, Size: 4},
		{Path: p2, Size: 4},
		{Path: p3, Size: 4},
	}
	var errs []ScanError
	classes := verifyGroup(group, &errs)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	// Expect two classes: {p1,p2} and {p3}.
	sizes := []int{}
	for _, c := range classes {
		sizes = append(sizes, len(c))
	}
	sort.Ints(sizes)
	if !equalInts(sizes, []int{1, 2}) {
		t.Fatalf("class sizes = %v, want [1 2]", sizes)
	}
}

func TestDryRunAndDelete(t *testing.T) {
	root, _, _ := dupPair(t)

	rep, _, err := FindDuplicates([]string{root}, Options{MinSize: 1})
	if err != nil {
		t.Fatalf("FindDuplicates: %v", err)
	}
	if len(rep.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(rep.Groups))
	}

	// Dry run: nothing removed.
	var out bytes.Buffer
	if err := DeleteDuplicates(rep, DeleteOptions{DryRun: true, Keep: "shortest-path", Out: &out}); err != nil {
		t.Fatalf("dry-run delete: %v", err)
	}
	for _, p := range rep.Groups[0].Paths {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("dry-run removed %s: %v", p, err)
		}
	}
	if !strings.Contains(out.String(), "would-delete") {
		t.Fatalf("dry-run output missing 'would-delete': %q", out.String())
	}

	// Real delete: exactly one survives, and it is the keeper (shortest path).
	keeper := shortestPath(rep.Groups[0].members)
	out.Reset()
	if err := DeleteDuplicates(rep, DeleteOptions{Keep: "shortest-path", Out: &out}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	surviving := 0
	for _, p := range rep.Groups[0].Paths {
		if _, err := os.Stat(p); err == nil {
			surviving++
		}
	}
	if surviving != 1 {
		t.Fatalf("expected exactly 1 surviving file, got %d", surviving)
	}
	if _, err := os.Stat(keeper.Path); err != nil {
		t.Fatalf("keeper %s was deleted: %v", keeper.Path, err)
	}
	data, err := os.ReadFile(keeper.Path)
	if err != nil {
		t.Fatalf("read keeper: %v", err)
	}
	if string(data) != dupPairContent {
		t.Fatalf("keeper content corrupted: %q", string(data))
	}
}

func TestJSONOutput(t *testing.T) {
	root, _, _ := basicTree(t)
	rep, _, err := FindDuplicates([]string{root}, Options{MinSize: 1})
	if err != nil {
		t.Fatalf("FindDuplicates: %v", err)
	}
	var buf bytes.Buffer
	if err := writeJSON(&buf, rep); err != nil {
		t.Fatalf("writeJSON: %v", err)
	}
	var decoded Report
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Summary.Groups != 1 {
		t.Fatalf("summary groups = %d, want 1", decoded.Summary.Groups)
	}
	if len(decoded.Groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(decoded.Groups))
	}
	g := decoded.Groups[0]
	wantWasted := g.Size * int64(len(g.Paths)-1)
	if g.Wasted != wantWasted {
		t.Fatalf("wasted = %d, want %d", g.Wasted, wantWasted)
	}
	if decoded.Summary.WastedBytes != wantWasted {
		t.Fatalf("summary wasted = %d, want %d", decoded.Summary.WastedBytes, wantWasted)
	}
}

func TestPermissionError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 000 does not deny reads on Windows; skipping")
	}
	root := t.TempDir()
	dup := "shared duplicate content here"
	writeFile(t, filepath.Join(root, "d1.txt"), dup)
	writeFile(t, filepath.Join(root, "d2.txt"), dup)
	bad := filepath.Join(root, "secret.txt")
	other := filepath.Join(root, "secret2.txt")
	writeFile(t, bad, "secret content unreadable")
	writeFile(t, other, "secret content unreadable")
	if err := os.Chmod(bad, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(bad, 0o644) })

	rep, _, err := FindDuplicates([]string{root}, Options{MinSize: 1})
	if err != nil {
		t.Fatalf("FindDuplicates returned fatal error: %v", err)
	}
	// The d1/d2 duplicate must still be found.
	foundDup := false
	for _, g := range rep.Groups {
		if len(g.Paths) == 2 && strings.Contains(g.Paths[0], "d1.txt") || strings.Contains(g.Paths[len(g.Paths)-1], "d2.txt") {
			foundDup = true
		}
	}
	if !foundDup {
		t.Fatalf("expected the readable duplicate to still be found; groups=%+v", rep.Groups)
	}
	if len(rep.Errors) == 0 {
		t.Fatalf("expected the unreadable file to be reported in Errors")
	}
}

func TestSymlinkNotFollowedByDefault(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation typically requires privilege on Windows; skipping")
	}
	root := t.TempDir()
	dup := "linked duplicate content"
	real := filepath.Join(root, "real.txt")
	realCopy := filepath.Join(root, "real_copy.txt")
	writeFile(t, real, dup)
	writeFile(t, realCopy, dup)
	link := filepath.Join(root, "link.txt")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	rep, _, err := FindDuplicates([]string{root}, Options{MinSize: 1})
	if err != nil {
		t.Fatalf("FindDuplicates: %v", err)
	}
	if len(rep.Groups) != 1 {
		t.Fatalf("expected 1 group (real+real_copy), got %d", len(rep.Groups))
	}
	for _, p := range rep.Groups[0].Paths {
		if strings.Contains(p, "link.txt") {
			t.Fatalf("symlink should not be hashed/reported: %v", rep.Groups[0].Paths)
		}
	}
}

func TestHardlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("hardlink dev/ino detection is Unix-only here")
	}
	root := t.TempDir()
	a := filepath.Join(root, "a.txt")
	b := filepath.Join(root, "b.txt")
	writeFile(t, a, "hardlinked content body")
	if err := os.Link(a, b); err != nil {
		t.Skipf("cannot create hardlink: %v", err)
	}

	rep, _, err := FindDuplicates([]string{root}, Options{MinSize: 1})
	if err != nil {
		t.Fatalf("FindDuplicates: %v", err)
	}
	if len(rep.Groups) != 1 || len(rep.Groups[0].Paths) != 2 {
		t.Fatalf("expected one group with the two hardlinks, got %+v", rep.Groups)
	}

	// Delete must refuse to remove the hardlink of the keeper: both survive.
	var out bytes.Buffer
	if err := DeleteDuplicates(rep, DeleteOptions{Keep: "shortest-path", Out: &out}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	for _, p := range []string{a, b} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("hardlink %s was deleted (should be preserved): %v", p, err)
		}
	}
	if !strings.Contains(out.String(), "hardlink") {
		t.Fatalf("expected hardlink skip note in output: %q", out.String())
	}
}

func TestRunExitCodes(t *testing.T) {
	root, _, _ := dupPair(t)

	// Duplicates found -> exit 1.
	var so, se bytes.Buffer
	if code := run([]string{root}, &so, &se); code != 1 {
		t.Fatalf("exit code with dupes = %d, want 1; stderr=%s", code, se.String())
	}

	// No paths -> usage error 2.
	so.Reset()
	se.Reset()
	if code := run([]string{}, &so, &se); code != 2 {
		t.Fatalf("exit code with no path = %d, want 2", code)
	}

	// Clean tree (no dupes) -> exit 0.
	clean := t.TempDir()
	writeFile(t, filepath.Join(clean, "only.txt"), "unique")
	so.Reset()
	se.Reset()
	if code := run([]string{clean}, &so, &se); code != 0 {
		t.Fatalf("exit code clean = %d, want 0", code)
	}

	// Nonexistent path -> runtime error 3.
	so.Reset()
	se.Reset()
	if code := run([]string{filepath.Join(root, "does-not-exist-xyz")}, &so, &se); code != 3 {
		t.Fatalf("exit code bad path = %d, want 3", code)
	}
}

func TestParseSize(t *testing.T) {
	cases := map[string]int64{
		"0":   0,
		"1":   1,
		"512": 512,
		"4k":  4096,
		"4K":  4096,
		"1M":  1 << 20,
		"2g":  2 << 30,
		"8kB": 8192,
	}
	for in, want := range cases {
		got, err := parseSize(in)
		if err != nil {
			t.Fatalf("parseSize(%q) error: %v", in, err)
		}
		if got != want {
			t.Fatalf("parseSize(%q) = %d, want %d", in, got, want)
		}
	}
	for _, bad := range []string{"", "abc", "-1", "12x"} {
		if _, err := parseSize(bad); err == nil {
			t.Fatalf("parseSize(%q) expected error", bad)
		}
	}
}

const dupPairContent = "this pair of files is identical content for delete tests"

func dupPair(t *testing.T) (root, p1, p2 string) {
	t.Helper()
	root = t.TempDir()
	p1 = filepath.Join(root, "longer-name-keep-or-not.txt")
	p2 = filepath.Join(root, "short.txt")
	writeFile(t, p1, dupPairContent)
	writeFile(t, p2, dupPairContent)
	return root, p1, p2
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
