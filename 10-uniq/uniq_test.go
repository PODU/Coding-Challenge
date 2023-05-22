package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tests for run.

func TestRun(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		in   string
		want string
	}{
		{
			name: "empty input",
			in:   "",
			want: "",
		},
		{
			name: "no duplicates default",
			in:   "a\nb\nc\n",
			want: "a\nb\nc\n",
		},
		{
			name: "adjacent duplicates collapsed",
			in:   "a\na\na\nb\nb\nc\n",
			want: "a\nb\nc\n",
		},
		{
			name: "non-adjacent duplicates preserved",
			in:   "a\nb\na\n",
			want: "a\nb\na\n",
		},
		{
			name: "count field width",
			cfg:  Config{Count: true},
			in:   "a\na\na\na\nb\nc\nc\n",
			want: "      4 a\n      1 b\n      2 c\n",
		},
		{
			name: "repeated only",
			cfg:  Config{DupesOnly: true},
			in:   "a\na\nb\nc\nc\nc\nd\n",
			want: "a\nc\n",
		},
		{
			name: "unique only",
			cfg:  Config{UniqOnly: true},
			in:   "a\na\nb\nc\nc\nc\nd\n",
			want: "b\nd\n",
		},
		{
			name: "count and repeated",
			cfg:  Config{Count: true, DupesOnly: true},
			in:   "a\na\nb\nc\nc\nc\n",
			want: "      2 a\n      3 c\n",
		},
		{
			name: "count and unique",
			cfg:  Config{Count: true, UniqOnly: true},
			in:   "a\na\nb\nc\nc\nc\nd\n",
			want: "      1 b\n      1 d\n",
		},
		{
			name: "repeated and unique is empty",
			cfg:  Config{DupesOnly: true, UniqOnly: true},
			in:   "a\na\nb\nc\nc\n",
			want: "",
		},
		{
			name: "count repeated and unique is empty",
			cfg:  Config{Count: true, DupesOnly: true, UniqOnly: true},
			in:   "a\na\nb\n",
			want: "",
		},
		{
			name: "no trailing newline final line emitted",
			in:   "a\nb\nb",
			want: "a\nb\n",
		},
		{
			name: "single line no trailing newline",
			in:   "solo",
			want: "solo\n",
		},
		{
			name: "blank line runs collapse",
			in:   "\n\n\nx\n\n",
			want: "\nx\n\n",
		},
		{
			name: "leading whitespace is significant",
			in:   " a\na\n a\n",
			want: " a\na\n a\n",
		},
		{
			name: "all identical default",
			in:   "x\nx\nx\n",
			want: "x\n",
		},
		{
			name: "all identical unique is empty",
			cfg:  Config{UniqOnly: true},
			in:   "x\nx\nx\n",
			want: "",
		},
		{
			name: "crlf preserved as part of line",
			in:   "a\r\na\r\nb\n",
			want: "a\r\nb\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := run(tc.cfg, strings.NewReader(tc.in), &out); err != nil {
				t.Fatalf("run returned error: %v", err)
			}
			if got := out.String(); got != tc.want {
				t.Errorf("run(%q)\n got: %q\nwant: %q", tc.in, got, tc.want)
			}
		})
	}
}

// A line longer than bufio.Scanner's default 64KB limit.
func TestRunLongLine(t *testing.T) {
	long := strings.Repeat("z", 200*1024) // 200 KB single line
	in := long + "\n" + long + "\n" + "tail\n"
	want := long + "\n" + "tail\n"

	var out bytes.Buffer
	if err := run(Config{}, strings.NewReader(in), &out); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	if got := out.String(); got != want {
		t.Errorf("long-line run mismatch: got %d bytes, want %d bytes", len(got), len(want))
	}
}

// parseArgs tests.

func TestParseArgs(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		want    parseResult
		wantErr bool
	}{
		{name: "no args", args: nil, want: parseResult{}},
		{name: "short count", args: []string{"-c"}, want: parseResult{cfg: Config{Count: true}}},
		{name: "bundled cd", args: []string{"-cd"}, want: parseResult{cfg: Config{Count: true, DupesOnly: true}}},
		{name: "bundled cdu", args: []string{"-cdu"}, want: parseResult{cfg: Config{Count: true, DupesOnly: true, UniqOnly: true}}},
		{name: "long forms", args: []string{"--count", "--repeated", "--unique"}, want: parseResult{cfg: Config{Count: true, DupesOnly: true, UniqOnly: true}}},
		{name: "input positional", args: []string{"in.txt"}, want: parseResult{inPath: "in.txt"}},
		{name: "input and output", args: []string{"in.txt", "out.txt"}, want: parseResult{inPath: "in.txt", outPath: "out.txt"}},
		{name: "flags after positionals", args: []string{"in.txt", "-c"}, want: parseResult{cfg: Config{Count: true}, inPath: "in.txt"}},
		{name: "dash is stdin positional", args: []string{"-"}, want: parseResult{inPath: "-"}},
		{name: "double dash terminates", args: []string{"--", "-c"}, want: parseResult{inPath: "-c"}},
		{name: "help short", args: []string{"-h"}, want: parseResult{help: true}},
		{name: "help long", args: []string{"--help"}, want: parseResult{help: true}},
		{name: "unknown short", args: []string{"-x"}, wantErr: true},
		{name: "unknown long", args: []string{"--nope"}, wantErr: true},
		{name: "too many positionals", args: []string{"a", "b", "c"}, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseArgs(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (result %+v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("parseArgs(%v) = %+v, want %+v", tc.args, got, tc.want)
			}
		})
	}
}

// End-to-end tests through realMain.

func TestRealMainStdin(t *testing.T) {
	in := strings.NewReader("a\na\nb\n")
	var out, errBuf bytes.Buffer
	code := realMain(nil, in, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errBuf.String())
	}
	if out.String() != "a\nb\n" {
		t.Errorf("stdout = %q, want %q", out.String(), "a\nb\n")
	}
}

func TestRealMainDashStdin(t *testing.T) {
	in := strings.NewReader("x\nx\ny\n")
	var out, errBuf bytes.Buffer
	if code := realMain([]string{"-c", "-"}, in, &out, &errBuf); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errBuf.String())
	}
	if out.String() != "      2 x\n      1 y\n" {
		t.Errorf("stdout = %q", out.String())
	}
}

func TestRealMainFileInput(t *testing.T) {
	dir := t.TempDir()
	inPath := filepath.Join(dir, "in.txt")
	if err := os.WriteFile(inPath, []byte("a\na\nb\nb\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	if code := realMain([]string{"-cd", inPath}, strings.NewReader(""), &out, &errBuf); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errBuf.String())
	}
	if out.String() != "      2 a\n      3 b\n" {
		t.Errorf("stdout = %q", out.String())
	}
}

func TestRealMainFileOutput(t *testing.T) {
	dir := t.TempDir()
	inPath := filepath.Join(dir, "in.txt")
	outPath := filepath.Join(dir, "out.txt")
	if err := os.WriteFile(inPath, []byte("a\na\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	if code := realMain([]string{inPath, outPath}, strings.NewReader(""), &out, &errBuf); code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, errBuf.String())
	}
	if out.Len() != 0 {
		t.Errorf("stdout should be empty, got %q", out.String())
	}
	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "a\nb\n" {
		t.Errorf("output file = %q, want %q", got, "a\nb\n")
	}
}

func TestRealMainMissingFile(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := realMain([]string{filepath.Join(t.TempDir(), "nope.txt")}, strings.NewReader(""), &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if out.Len() != 0 {
		t.Errorf("stdout should be empty on error, got %q", out.String())
	}
	if errBuf.Len() == 0 {
		t.Errorf("expected an error message on stderr")
	}
}

func TestRealMainUnknownFlag(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := realMain([]string{"-z"}, strings.NewReader("a\n"), &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "Usage:") {
		t.Errorf("expected usage on stderr, got %q", errBuf.String())
	}
}

func TestRealMainHelp(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := realMain([]string{"--help"}, strings.NewReader(""), &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Errorf("expected usage on stdout, got %q", out.String())
	}
}

func TestRealMainRefusesSamePath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(p, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	code := realMain([]string{p, p}, strings.NewReader(""), &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "same file") {
		t.Errorf("expected same-file refusal, got %q", errBuf.String())
	}
}
