package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile creates a temp file with the given content and returns its path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

const sample = "hello\nworld\nfrom cat\n"

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCfg  config
		wantOps  []string
		wantErr  bool
		wantHelp bool
	}{
		{name: "no args", args: nil, wantOps: []string{}},
		{name: "single file", args: []string{"a.txt"}, wantOps: []string{"a.txt"}},
		{name: "dash is stdin operand", args: []string{"-"}, wantOps: []string{"-"}},
		{name: "n flag", args: []string{"-n", "a"}, wantCfg: config{number: true}, wantOps: []string{"a"}},
		{name: "b flag", args: []string{"-b", "a"}, wantCfg: config{numberNonBlank: true}, wantOps: []string{"a"}},
		{name: "combined nb", args: []string{"-nb", "a"}, wantCfg: config{number: true, numberNonBlank: true}, wantOps: []string{"a"}},
		{name: "separate n b", args: []string{"-n", "-b", "a"}, wantCfg: config{number: true, numberNonBlank: true}, wantOps: []string{"a"}},
		{name: "double dash terminator", args: []string{"--", "-n"}, wantOps: []string{"-n"}},
		{name: "mix files and dash", args: []string{"-n", "a", "-", "b"}, wantCfg: config{number: true}, wantOps: []string{"a", "-", "b"}},
		{name: "unknown flag", args: []string{"-z"}, wantErr: true},
		{name: "help short", args: []string{"-h"}, wantHelp: true},
		{name: "help long", args: []string{"--help"}, wantHelp: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, ops, err := parseArgs(tc.args)
			if tc.wantHelp {
				if err != errHelp {
					t.Fatalf("want errHelp, got %v", err)
				}
				return
			}
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg != tc.wantCfg {
				t.Errorf("cfg = %+v, want %+v", cfg, tc.wantCfg)
			}
			if strings.Join(ops, ",") != strings.Join(tc.wantOps, ",") {
				t.Errorf("ops = %v, want %v", ops, tc.wantOps)
			}
		})
	}
}

func TestRunGolden(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.txt", "alpha\nbeta\n")
	b := writeFile(t, dir, "b.txt", "gamma\n")
	blanks := writeFile(t, dir, "blanks.txt", "one\n\ntwo\n\n")
	noNL := writeFile(t, dir, "nonl.txt", "no newline")
	empty := writeFile(t, dir, "empty.txt", "")
	single := writeFile(t, dir, "single.txt", sample)

	tests := []struct {
		name  string
		args  []string
		stdin string
		want  string
		exit  int
	}{
		{
			name: "single file step1",
			args: []string{single},
			want: sample,
		},
		{
			name: "concatenate order step3",
			args: []string{a, b},
			want: "alpha\nbeta\ngamma\n",
		},
		{
			name:  "stdin via dash step2",
			args:  []string{"-"},
			stdin: sample,
			want:  sample,
		},
		{
			name:  "stdin no args",
			args:  nil,
			stdin: "piped input\n",
			want:  "piped input\n",
		},
		{
			name: "number all lines step4",
			args: []string{"-n", a},
			want: "     1\talpha\n     2\tbeta\n",
		},
		{
			name: "number all includes blanks",
			args: []string{"-n", blanks},
			want: "     1\tone\n     2\t\n     3\ttwo\n     4\t\n",
		},
		{
			name: "number non-blank step5",
			args: []string{"-b", blanks},
			want: "     1\tone\n\n     2\ttwo\n\n",
		},
		{
			name: "b overrides n",
			args: []string{"-n", "-b", blanks},
			want: "     1\tone\n\n     2\ttwo\n\n",
		},
		{
			name: "continuous numbering across files",
			args: []string{"-n", a, b},
			want: "     1\talpha\n     2\tbeta\n     3\tgamma\n",
		},
		{
			name: "no trailing newline preserved",
			args: []string{noNL},
			want: "no newline",
		},
		{
			name: "no trailing newline numbered",
			args: []string{"-n", noNL},
			want: "     1\tno newline",
		},
		{
			name: "empty file produces nothing",
			args: []string{empty},
			want: "",
		},
		{
			name: "empty file consumes no number",
			args: []string{"-n", empty, a},
			want: "     1\talpha\n     2\tbeta\n",
		},
		{
			name:  "mix file stdin file continuous",
			args:  []string{"-n", a, "-", b},
			stdin: "MID\n",
			want:  "     1\talpha\n     2\tbeta\n     3\tMID\n     4\tgamma\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out, errBuf bytes.Buffer
			code := run(tc.args, strings.NewReader(tc.stdin), &out, &errBuf)
			if code != tc.exit {
				t.Errorf("exit = %d, want %d (stderr: %q)", code, tc.exit, errBuf.String())
			}
			if out.String() != tc.want {
				t.Errorf("out = %q, want %q", out.String(), tc.want)
			}
		})
	}
}

func TestRunMissingFileContinues(t *testing.T) {
	dir := t.TempDir()
	good := writeFile(t, dir, "good.txt", "good\n")
	missing := filepath.Join(dir, "does-not-exist.txt")

	var out, errBuf bytes.Buffer
	code := run([]string{missing, good}, strings.NewReader(""), &out, &errBuf)

	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if out.String() != "good\n" {
		t.Errorf("out = %q, want %q (remaining file must still print)", out.String(), "good\n")
	}
	if !strings.Contains(errBuf.String(), "ccat:") || !strings.Contains(errBuf.String(), "No such file") {
		t.Errorf("stderr = %q, want a 'No such file' diagnostic", errBuf.String())
	}
}

func TestRunDirectoryArg(t *testing.T) {
	dir := t.TempDir()

	var out, errBuf bytes.Buffer
	code := run([]string{dir}, strings.NewReader(""), &out, &errBuf)

	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if out.Len() != 0 {
		t.Errorf("out = %q, want empty (must not dump directory)", out.String())
	}
	if !strings.Contains(errBuf.String(), "Is a directory") {
		t.Errorf("stderr = %q, want 'Is a directory'", errBuf.String())
	}
}

func TestRunUnknownFlag(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"-z"}, strings.NewReader(""), &out, &errBuf)

	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "invalid option") {
		t.Errorf("stderr = %q, want 'invalid option'", errBuf.String())
	}
}

func TestRunHelp(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"--help"}, strings.NewReader(""), &out, &errBuf)

	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "Usage: ccat") {
		t.Errorf("out = %q, want usage text", out.String())
	}
}

func TestRunDoubleDash(t *testing.T) {
	dir := t.TempDir()
	// A file literally named "-n" must be readable via "--".
	dashFile := writeFile(t, dir, "-n", "literal\n")

	var out, errBuf bytes.Buffer
	code := run([]string{"--", dashFile}, strings.NewReader(""), &out, &errBuf)

	if code != 0 {
		t.Errorf("exit = %d, want 0 (stderr: %q)", code, errBuf.String())
	}
	if out.String() != "literal\n" {
		t.Errorf("out = %q, want %q", out.String(), "literal\n")
	}
}
