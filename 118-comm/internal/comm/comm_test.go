package comm

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// td returns the path to a fixture under testdata/.
func td(name string) string { return filepath.Join("testdata", name) }

// runComm executes Run with the given args and optional stdin, capturing
// stdout, stderr, and the exit code.
func runComm(t *testing.T, stdin io.Reader, args ...string) (string, string, int) {
	t.Helper()
	if stdin == nil {
		stdin = strings.NewReader("")
	}
	var out, errBuf bytes.Buffer
	code := Run(args, stdin, &out, &errBuf)
	return out.String(), errBuf.String(), code
}

// TestGolden covers the column-placement scenarios with exact byte output.
func TestGolden(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		stdin io.Reader
		want  string
		code  int
	}{
		{
			name: "baseline three columns",
			args: []string{td("a.txt"), td("b.txt")},
			want: "apple\n\t\tbanana\n\t\tcherry\ndate\n\telderberry\n\tfig\n",
		},
		{
			name: "suppress 1",
			args: []string{"-1", td("a.txt"), td("b.txt")},
			want: "\tbanana\n\tcherry\nelderberry\nfig\n",
		},
		{
			name: "suppress 2",
			args: []string{"-2", td("a.txt"), td("b.txt")},
			want: "apple\n\tbanana\n\tcherry\ndate\n",
		},
		{
			name: "suppress 3",
			args: []string{"-3", td("a.txt"), td("b.txt")},
			want: "apple\ndate\n\telderberry\n\tfig\n",
		},
		{
			name: "suppress 12 common only",
			args: []string{"-12", td("a.txt"), td("b.txt")},
			want: "banana\ncherry\n",
		},
		{
			name: "suppress 13 only file2",
			args: []string{"-13", td("a.txt"), td("b.txt")},
			want: "elderberry\nfig\n",
		},
		{
			name: "suppress 23 only file1",
			args: []string{"-23", td("a.txt"), td("b.txt")},
			want: "apple\ndate\n",
		},
		{
			name: "suppress 123 no rows",
			args: []string{"-123", td("a.txt"), td("b.txt")},
			want: "",
		},
		{
			name: "suppress 1 and 2 as separate args",
			args: []string{"-1", "-2", td("a.txt"), td("b.txt")},
			want: "banana\ncherry\n",
		},
		{
			name: "equal files all common",
			args: []string{td("equal1.txt"), td("equal2.txt")},
			want: "\t\talpha\n\t\tbeta\n\t\tgamma\n",
		},
		{
			name: "equal files suppress 3 empty",
			args: []string{"-3", td("equal1.txt"), td("equal2.txt")},
			want: "",
		},
		{
			name: "disjoint files interleave",
			args: []string{td("disjoint1.txt"), td("disjoint2.txt")},
			want: "ant\n\tbee\ncat\n\tdog\neel\n\tfox\n",
		},
		{
			name: "empty paired with nonempty",
			args: []string{td("empty.txt"), td("a.txt")},
			want: "\tapple\n\tbanana\n\tcherry\n\tdate\n",
		},
		{
			name: "two empty files no output",
			args: []string{td("empty.txt"), td("empty.txt")},
			want: "",
		},
		{
			name: "duplicate lines positional pairing",
			args: []string{td("dups1.txt"), td("dups2.txt")},
			want: "\t\tapple\napple\n\t\tbanana\n\tbanana\n",
		},
		{
			name: "case sensitive default",
			args: []string{td("case1.txt"), td("case2.txt")},
			want: "Apple\nBanana\n\tapple\n\tbanana\n",
		},
		{
			name: "case insensitive -i preserves file1 case",
			args: []string{"-i", td("case1.txt"), td("case2.txt")},
			want: "\t\tApple\n\t\tBanana\n",
		},
		{
			name: "output delimiter comma",
			args: []string{"--output-delimiter=,", td("a.txt"), td("b.txt")},
			want: "apple\n,,banana\n,,cherry\ndate\n,elderberry\n,fig\n",
		},
		{
			name: "total summary",
			args: []string{"--total", td("a.txt"), td("b.txt")},
			want: "apple\n\t\tbanana\n\t\tcherry\ndate\n\telderberry\n\tfig\n2\t2\t2\ttotal\n",
		},
		{
			name: "total with suppress 123",
			args: []string{"-123", "--total", td("a.txt"), td("b.txt")},
			want: "2\t2\t2\ttotal\n",
		},
		{
			name:  "stdin as file1",
			args:  []string{"-", td("b.txt")},
			stdin: strings.NewReader("apple\nbanana\ncherry\ndate\n"),
			want:  "apple\n\t\tbanana\n\t\tcherry\ndate\n\telderberry\n\tfig\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, _, code := runComm(t, tt.stdin, tt.args...)
			if code != tt.code {
				t.Fatalf("exit code = %d, want %d", code, tt.code)
			}
			if out != tt.want {
				t.Errorf("output mismatch\n got: %q\nwant: %q", out, tt.want)
			}
		})
	}
}

func TestBothStdinError(t *testing.T) {
	_, errOut, code := runComm(t, strings.NewReader(""), "-", "-")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut, "standard input") {
		t.Errorf("stderr = %q, want mention of standard input", errOut)
	}
}

func TestOrderChecking(t *testing.T) {
	t.Run("default warns once exit 0", func(t *testing.T) {
		_, errOut, code := runComm(t, nil, td("unsorted.txt"), td("b.txt"))
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		if !strings.Contains(errOut, "file 1 is not in sorted order") {
			t.Errorf("stderr = %q, want sorted-order warning", errOut)
		}
		if strings.Count(errOut, "not in sorted order") != 1 {
			t.Errorf("expected exactly one warning, got %q", errOut)
		}
	})

	t.Run("check-order fatal exit 1", func(t *testing.T) {
		_, errOut, code := runComm(t, nil, "--check-order", td("unsorted.txt"), td("b.txt"))
		if code != 1 {
			t.Fatalf("exit code = %d, want 1", code)
		}
		if !strings.Contains(errOut, "not in sorted order") {
			t.Errorf("stderr = %q, want sorted-order error", errOut)
		}
	})

	t.Run("nocheck-order silent exit 0", func(t *testing.T) {
		_, errOut, code := runComm(t, nil, "--nocheck-order", td("unsorted.txt"), td("b.txt"))
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		if errOut != "" {
			t.Errorf("stderr = %q, want empty", errOut)
		}
	})
}

func TestUsageErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"one positional", []string{td("a.txt")}},
		{"three positionals", []string{td("a.txt"), td("b.txt"), td("a.txt")}},
		{"unknown long flag", []string{"--frobnicate", td("a.txt"), td("b.txt")}},
		{"unknown short flag", []string{"-x", td("a.txt"), td("b.txt")}},
		{"empty delimiter", []string{"--output-delimiter=", td("a.txt"), td("b.txt")}},
		{"missing file", []string{td("a.txt"), td("does-not-exist.txt")}},
		{"check and nocheck", []string{"--check-order", "--nocheck-order", td("a.txt"), td("b.txt")}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, code := runComm(t, nil, c.args...)
			if code != 1 {
				t.Errorf("exit code = %d, want 1", code)
			}
		})
	}
}

func TestHelp(t *testing.T) {
	out, _, code := runComm(t, nil, "--help")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "Usage: comm") {
		t.Errorf("help output missing usage line: %q", out)
	}
}

// TestLastLineWithoutNewline verifies a final line lacking '\n' is emitted and
// not given a spurious trailing newline beyond the per-row one.
func TestLastLineWithoutNewline(t *testing.T) {
	a := strings.NewReader("x")
	dir := t.TempDir()
	bPath := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(bPath, []byte("y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, code := runComm(t, a, "-", bPath)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	// x (only file1), then y (only file2).
	want := "x\n\ty\n"
	if out != want {
		t.Errorf("output = %q, want %q", out, want)
	}
}

func TestCompareLines(t *testing.T) {
	tests := []struct {
		a, b string
		ci   bool
		want int
	}{
		{"a", "b", false, -1},
		{"b", "a", false, 1},
		{"a", "a", false, 0},
		{"Apple", "apple", false, -1}, // 'A'(65) < 'a'(97)
		{"Apple", "apple", true, 0},
		{"BANANA", "banana", true, 0},
		{"apple", "apricot", true, -1},
		{"abc", "abcd", false, -1},
	}
	for _, tt := range tests {
		if got := compareLines([]byte(tt.a), []byte(tt.b), tt.ci); got != tt.want {
			t.Errorf("compareLines(%q,%q,ci=%v) = %d, want %d", tt.a, tt.b, tt.ci, got, tt.want)
		}
	}
}
