package grep

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mygrep/internal/cli"
)

// run executes Run with the given stdin text and returns stdout, stderr, exit.
func run(t *testing.T, cfg *cli.Config, stdin string) (string, string, int) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := Run(cfg, strings.NewReader(stdin), &out, &errOut)
	return out.String(), errOut.String(), code
}

// writeFile creates path under dir with content and returns the full path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestStdinMatching(t *testing.T) {
	in := "alpha\nbeto\ngamma\n" // only alpha and gamma contain "a"
	out, errOut, code := run(t, &cli.Config{Pattern: "a"}, in)
	if out != "alpha\ngamma\n" {
		t.Errorf("stdout = %q", out)
	}
	if errOut != "" {
		t.Errorf("stderr = %q", errOut)
	}
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
}

func TestEmptyPatternMatchesEveryLine(t *testing.T) {
	in := "one\ntwo\nthree\n"
	out, _, code := run(t, &cli.Config{Pattern: ""}, in)
	if out != in {
		t.Errorf("stdout = %q, want %q", out, in)
	}
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
}

func TestNoMatchExit1(t *testing.T) {
	out, _, code := run(t, &cli.Config{Pattern: "zzzzz"}, "abc\ndef\n")
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
}

func TestInvert(t *testing.T) {
	in := "error here\nall good\nerror again\n"
	out, _, code := run(t, &cli.Config{Pattern: "error", Invert: true}, in)
	if out != "all good\n" {
		t.Errorf("stdout = %q", out)
	}
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
}

func TestInvertCountEmptyPattern(t *testing.T) {
	// Empty pattern matches all, so inverted count is 0.
	out, _, code := run(t, &cli.Config{Pattern: "", Invert: true, Count: true}, "a\nb\nc\n")
	if out != "0\n" {
		t.Errorf("stdout = %q, want %q", out, "0\n")
	}
	if code != 1 {
		t.Errorf("exit = %d, want 1 (nothing selected)", code)
	}
}

func TestCount(t *testing.T) {
	in := "x1\ny\nz9\nw\n5\n"
	out, _, code := run(t, &cli.Config{Pattern: `\d`, Count: true}, in)
	if out != "3\n" {
		t.Errorf("stdout = %q, want %q", out, "3\n")
	}
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
}

func TestLineNumbers(t *testing.T) {
	in := "The start\nmiddle\nThe end\n"
	out, _, _ := run(t, &cli.Config{Pattern: "^The", LineNumbers: true}, in)
	want := "1:The start\n3:The end\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

func TestCRLFStrippedForAnchor(t *testing.T) {
	// "$" must match even when the file uses Windows CRLF endings.
	in := "hello\r\nworld\r\n"
	out, _, _ := run(t, &cli.Config{Pattern: "hello$"}, in)
	if out != "hello\n" {
		t.Errorf("stdout = %q, want %q", out, "hello\n")
	}
}

func TestNoTrailingNewline(t *testing.T) {
	out, _, code := run(t, &cli.Config{Pattern: "tail"}, "tail end no newline")
	if out != "tail end no newline\n" {
		t.Errorf("stdout = %q", out)
	}
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
}

func TestVeryLongLine(t *testing.T) {
	long := strings.Repeat("a", 500_000) + "NEEDLE" + strings.Repeat("b", 500_000)
	out, _, code := run(t, &cli.Config{Pattern: "NEEDLE", Count: true}, long+"\n")
	if out != "1\n" {
		t.Errorf("stdout = %q, want 1", out)
	}
	if code != 0 {
		t.Errorf("exit = %d", code)
	}
}

func TestSingleFileNoPrefix(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "a.txt", "cat\ndog\n")
	out, _, code := run(t, &cli.Config{Pattern: "cat", Paths: []string{p}}, "")
	if out != "cat\n" {
		t.Errorf("stdout = %q, want %q (no prefix for single file)", out, "cat\n")
	}
	if code != 0 {
		t.Errorf("exit = %d", code)
	}
}

func TestMultipleFilesPrefix(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.txt", "cat\n")
	b := writeFile(t, dir, "b.txt", "cat\n")
	out, _, _ := run(t, &cli.Config{Pattern: "cat", Paths: []string{a, b}}, "")
	want := a + ":cat\n" + b + ":cat\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

func TestCountMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.txt", "x\nxx\n")
	b := writeFile(t, dir, "b.txt", "y\n")
	out, _, _ := run(t, &cli.Config{Pattern: "x", Count: true, Paths: []string{a, b}}, "")
	want := a + ":2\n" + b + ":0\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

func TestRecursive(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "top.txt", "no hits here\n")
	sub := writeFile(t, dir, filepath.Join("subdir", "BFS1985.txt"), "released in 1985\nother\n")
	out, _, code := run(t, &cli.Config{Pattern: "1985", Recursive: true, Paths: []string{dir}}, "")
	wantLine := filepath.ToSlash(sub) + ":released in 1985\n"
	if out != wantLine {
		t.Errorf("stdout = %q, want %q", out, wantLine)
	}
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
}

func TestDirectoryWithoutRecursive(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "hello\n")
	out, errOut, code := run(t, &cli.Config{Pattern: "hello", Paths: []string{dir}}, "")
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
	if !strings.Contains(errOut, "Is a directory") {
		t.Errorf("stderr = %q, want 'Is a directory'", errOut)
	}
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}

func TestMissingFile(t *testing.T) {
	out, errOut, code := run(t, &cli.Config{Pattern: "x", Paths: []string{"does-not-exist.txt"}}, "")
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
	if !strings.Contains(errOut, "No such file or directory") {
		t.Errorf("stderr = %q", errOut)
	}
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}

func TestErrorTakesPrecedenceOverMatch(t *testing.T) {
	// A real match plus a missing file: exit must still be 2.
	dir := t.TempDir()
	a := writeFile(t, dir, "a.txt", "match\n")
	_, _, code := run(t, &cli.Config{Pattern: "match", Paths: []string{a, "nope.txt"}}, "")
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}

func TestBadRegexExit2(t *testing.T) {
	_, errOut, code := run(t, &cli.Config{Pattern: "("}, "anything\n")
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errOut, "invalid pattern") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestWordMatch(t *testing.T) {
	in := "cat\ncategory\nthe cat sat\nconcatenate\n"
	out, _, _ := run(t, &cli.Config{Pattern: "cat", WordMatch: true}, in)
	want := "cat\nthe cat sat\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

func TestCaseInsensitive(t *testing.T) {
	in := "Nirvana\nNIRVANA\nPearl Jam\nnirvana live\n"
	out, _, _ := run(t, &cli.Config{Pattern: "nirvana", IgnoreCase: true, Count: true}, in)
	if out != "3\n" {
		t.Errorf("stdout = %q, want 3", out)
	}
}

func TestEmptyFile(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "empty.txt", "")
	out, _, code := run(t, &cli.Config{Pattern: "x", Paths: []string{p}}, "")
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
}

func TestDashIsStdin(t *testing.T) {
	out, _, code := run(t, &cli.Config{Pattern: "a", Paths: []string{"-"}}, "abc\nxyz\n")
	if out != "abc\n" {
		t.Errorf("stdout = %q", out)
	}
	if code != 0 {
		t.Errorf("exit = %d", code)
	}
}
