package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- Core copy-function unit tests ----

func TestCopyFirstLines(t *testing.T) {
	tests := []struct {
		name string
		in   string
		n    int64
		want string
	}{
		{"fewer than file", "a\nb\nc\nd\n", 2, "a\nb\n"},
		{"equal to file", "a\nb\nc\n", 3, "a\nb\nc\n"},
		{"more than file", "a\nb\n", 5, "a\nb\n"},
		{"zero", "a\nb\n", 0, ""},
		{"no trailing newline", "a\nb\nc", 3, "a\nb\nc"},
		{"partial last line counted", "a\nb\nc", 2, "a\nb\n"},
		{"empty input", "", 3, ""},
		{"crlf preserved", "a\r\nb\r\n", 1, "a\r\n"},
		{"blank lines", "\n\n\n", 2, "\n\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var w bytes.Buffer
			if err := copyFirstLines(&w, strings.NewReader(tc.in), tc.n); err != nil {
				t.Fatalf("err: %v", err)
			}
			if w.String() != tc.want {
				t.Errorf("got %q want %q", w.String(), tc.want)
			}
		})
	}
}

func TestCopyFirstBytes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		n    int64
		want string
	}{
		{"fewer", "hello world", 5, "hello"},
		{"equal", "hello", 5, "hello"},
		{"more", "hi", 10, "hi"},
		{"zero", "hello", 0, ""},
		{"multibyte split", "caf\xc3\xa9", 4, "caf\xc3"},
		{"binary", "\x00\x01\x02\x03", 2, "\x00\x01"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var w bytes.Buffer
			if err := copyFirstBytes(&w, strings.NewReader(tc.in), tc.n); err != nil {
				t.Fatalf("err: %v", err)
			}
			if w.String() != tc.want {
				t.Errorf("got %q want %q", w.String(), tc.want)
			}
		})
	}
}

func TestCopyAllButLastLines(t *testing.T) {
	tests := []struct {
		name string
		in   string
		k    int64
		want string
	}{
		{"k below total", "a\nb\nc\nd\n", 1, "a\nb\nc\n"},
		{"k equals total", "a\nb\nc\n", 3, ""},
		{"k above total", "a\nb\n", 5, ""},
		{"k zero emits all", "a\nb\n", 0, "a\nb\n"},
		{"no trailing newline", "a\nb\nc", 1, "a\nb\n"},
		{"empty", "", 2, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var w bytes.Buffer
			if err := copyAllButLastLines(&w, strings.NewReader(tc.in), tc.k); err != nil {
				t.Fatalf("err: %v", err)
			}
			if w.String() != tc.want {
				t.Errorf("got %q want %q", w.String(), tc.want)
			}
		})
	}
}

func TestCopyAllButLastBytes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		k    int64
		want string
	}{
		{"k below size", "hello", 2, "hel"},
		{"k equals size", "hello", 5, ""},
		{"k above size", "hi", 5, ""},
		{"k zero emits all", "hello", 0, "hello"},
		{"binary", "\x00\x01\x02\x03", 1, "\x00\x01\x02"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var w bytes.Buffer
			if err := copyAllButLastBytes(&w, strings.NewReader(tc.in), tc.k); err != nil {
				t.Fatalf("err: %v", err)
			}
			if w.String() != tc.want {
				t.Errorf("got %q want %q", w.String(), tc.want)
			}
		})
	}
}

// Large input must stream with a small window for the bytes case.
func TestCopyAllButLastBytesLarge(t *testing.T) {
	in := strings.Repeat("0123456789", 100000) // 1,000,000 bytes
	var w bytes.Buffer
	if err := copyAllButLastBytes(&w, strings.NewReader(in), 3); err != nil {
		t.Fatal(err)
	}
	if w.Len() != len(in)-3 {
		t.Fatalf("got len %d want %d", w.Len(), len(in)-3)
	}
	if w.String() != in[:len(in)-3] {
		t.Error("content mismatch")
	}
}

// ---- Count parser tests ----

func TestParseCount(t *testing.T) {
	tests := []struct {
		in      string
		want    int64
		wantNeg bool
		wantErr bool
	}{
		{"5", 5, false, false},
		{"-3", 3, true, false},
		{"+5", 5, false, false},
		{"0", 0, false, false},
		{"10", 10, false, false},
		{"2b", 1024, false, false},
		{"1k", 1024, false, false},
		{"2k", 2048, false, false},
		{"1m", 1024 * 1024, false, false},
		{"", 0, false, true},
		{"abc", 0, false, true},
		{"5x", 0, false, true},
		{"-", 0, false, true},
		{"99999999999999999999999", 0, false, true},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			n, neg, err := parseCount(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if n != tc.want || neg != tc.wantNeg {
				t.Errorf("got (%d,%v) want (%d,%v)", n, neg, tc.want, tc.wantNeg)
			}
		})
	}
}

// ---- Argument parsing tests ----

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantMode  mode
		wantCount int64
		wantNeg   bool
		wantHdr   headerMode
		wantOps   []string
		wantErr   bool
	}{
		{"default", []string{"f.txt"}, modeLines, 10, false, headerAuto, []string{"f.txt"}, false},
		{"-n5 attached", []string{"-n5", "f"}, modeLines, 5, false, headerAuto, []string{"f"}, false},
		{"-n 5 sep", []string{"-n", "5", "f"}, modeLines, 5, false, headerAuto, []string{"f"}, false},
		{"-n=5", []string{"-n=5", "f"}, modeLines, 5, false, headerAuto, []string{"f"}, false},
		{"--lines=5", []string{"--lines=5", "f"}, modeLines, 5, false, headerAuto, []string{"f"}, false},
		{"--lines 5", []string{"--lines", "5", "f"}, modeLines, 5, false, headerAuto, []string{"f"}, false},
		{"-c 31", []string{"-c", "31", "f"}, modeBytes, 31, false, headerAuto, []string{"f"}, false},
		{"negative -n -3", []string{"-n", "-3", "f"}, modeLines, 3, true, headerAuto, []string{"f"}, false},
		{"last wins n then c", []string{"-n", "5", "-c", "9", "f"}, modeBytes, 9, false, headerAuto, []string{"f"}, false},
		{"last wins c then n", []string{"-c", "9", "-n", "5", "f"}, modeLines, 5, false, headerAuto, []string{"f"}, false},
		{"quiet", []string{"-q", "f"}, modeLines, 10, false, headerNever, []string{"f"}, false},
		{"verbose", []string{"-v", "f"}, modeLines, 10, false, headerAlways, []string{"f"}, false},
		{"cluster -qv", []string{"-qv", "f"}, modeLines, 10, false, headerAlways, []string{"f"}, false},
		{"dash operand", []string{"-", "f"}, modeLines, 10, false, headerAuto, []string{"-", "f"}, false},
		{"double dash", []string{"--", "-n", "f"}, modeLines, 10, false, headerAuto, []string{"-n", "f"}, false},
		{"no operands", []string{"-n", "3"}, modeLines, 3, false, headerAuto, nil, false},
		{"bad flag", []string{"-z"}, 0, 0, false, 0, nil, true},
		{"bad number", []string{"-n", "abc"}, 0, 0, false, 0, nil, true},
		{"missing arg", []string{"-n"}, 0, 0, false, 0, nil, true},
		{"unknown long", []string{"--bogus"}, 0, 0, false, 0, nil, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, ops, action, err := parseArgs(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if action != actionRun {
				t.Fatalf("unexpected action %v", action)
			}
			if cfg.m != tc.wantMode || cfg.count != tc.wantCount || cfg.negative != tc.wantNeg || cfg.headers != tc.wantHdr {
				t.Errorf("cfg got %+v", cfg)
			}
			if strings.Join(ops, ",") != strings.Join(tc.wantOps, ",") {
				t.Errorf("ops got %v want %v", ops, tc.wantOps)
			}
		})
	}
}

func TestParseArgsHelpVersion(t *testing.T) {
	for _, a := range [][]string{{"-h"}, {"--help"}} {
		_, _, action, err := parseArgs(a)
		if err != nil || action != actionHelp {
			t.Errorf("%v -> action %v err %v", a, action, err)
		}
	}
	_, _, action, err := parseArgs([]string{"--version"})
	if err != nil || action != actionVersion {
		t.Errorf("version -> action %v err %v", action, err)
	}
}

// ---- End-to-end run() golden tests ----

func run3(t *testing.T, args []string, stdin string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := run(args, strings.NewReader(stdin), &out, &errOut)
	return code, out.String(), errOut.String()
}

func td(name string) string { return filepath.Join("testdata", name) }

func TestRunDefaultTenLines(t *testing.T) {
	code, out, _ := run3(t, []string{td("multiline.txt")}, "")
	want := "line 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7\nline 8\nline 9\nline 10\n"
	if code != 0 || out != want {
		t.Errorf("code=%d out=%q", code, out)
	}
}

func TestRunNFlag(t *testing.T) {
	code, out, _ := run3(t, []string{"-n", "5", td("multiline.txt")}, "")
	if code != 0 || out != "line 1\nline 2\nline 3\nline 4\nline 5\n" {
		t.Errorf("code=%d out=%q", code, out)
	}
}

func TestRunN1Attached(t *testing.T) {
	code, out, _ := run3(t, []string{"-n1", td("short.txt")}, "")
	if code != 0 || out != "alpha\n" {
		t.Errorf("code=%d out=%q", code, out)
	}
}

func TestRunNZero(t *testing.T) {
	code, out, _ := run3(t, []string{"-n", "0", td("short.txt")}, "")
	if code != 0 || out != "" {
		t.Errorf("code=%d out=%q", code, out)
	}
}

func TestRunNegativeLines(t *testing.T) {
	code, out, _ := run3(t, []string{"-n", "-2", td("short.txt")}, "")
	if code != 0 || out != "alpha\n" {
		t.Errorf("code=%d out=%q", code, out)
	}
}

func TestRunCBytes(t *testing.T) {
	code, out, _ := run3(t, []string{"-c", "5", td("short.txt")}, "")
	if code != 0 || out != "alpha" {
		t.Errorf("code=%d out=%q", code, out)
	}
}

func TestRunNegativeBytes(t *testing.T) {
	// short.txt = "alpha\nbeta\ngamma\n" (17 bytes); all but last 6 -> 11 bytes
	code, out, _ := run3(t, []string{"-c", "-6", td("short.txt")}, "")
	if code != 0 || out != "alpha\nbeta\n" {
		t.Errorf("code=%d out=%q", code, out)
	}
}

func TestRunShortFileNoError(t *testing.T) {
	code, out, errOut := run3(t, []string{"-n", "100", td("short.txt")}, "")
	if code != 0 || out != "alpha\nbeta\ngamma\n" || errOut != "" {
		t.Errorf("code=%d out=%q err=%q", code, out, errOut)
	}
}

func TestRunNoNewlineFile(t *testing.T) {
	code, out, _ := run3(t, []string{td("nonewline.txt")}, "")
	if code != 0 || out != "first\nsecond\nthird" {
		t.Errorf("code=%d out=%q", code, out)
	}
}

func TestRunEmptyFile(t *testing.T) {
	code, out, _ := run3(t, []string{td("empty.txt")}, "")
	if code != 0 || out != "" {
		t.Errorf("code=%d out=%q", code, out)
	}
}

func TestRunStdin(t *testing.T) {
	code, out, _ := run3(t, []string{"-n", "2"}, "x\ny\nz\n")
	if code != 0 || out != "x\ny\n" {
		t.Errorf("code=%d out=%q", code, out)
	}
}

func TestRunDashStdin(t *testing.T) {
	code, out, _ := run3(t, []string{"-n", "1", "-"}, "only\nmore\n")
	if code != 0 || out != "only\n" {
		t.Errorf("code=%d out=%q", code, out)
	}
}

func TestRunMultiFileHeaders(t *testing.T) {
	code, out, _ := run3(t, []string{"-n", "1", td("short.txt"), td("multiline.txt")}, "")
	want := "==> " + td("short.txt") + " <==\nalpha\n\n==> " + td("multiline.txt") + " <==\nline 1\n"
	if code != 0 || out != want {
		t.Errorf("code=%d\n got=%q\nwant=%q", code, out, want)
	}
}

func TestRunQuietMultiFile(t *testing.T) {
	code, out, _ := run3(t, []string{"-q", "-n", "1", td("short.txt"), td("multiline.txt")}, "")
	if code != 0 || out != "alpha\nline 1\n" {
		t.Errorf("code=%d out=%q", code, out)
	}
}

func TestRunVerboseSingleFile(t *testing.T) {
	code, out, _ := run3(t, []string{"-v", "-n", "1", td("short.txt")}, "")
	want := "==> " + td("short.txt") + " <==\nalpha\n"
	if code != 0 || out != want {
		t.Errorf("code=%d out=%q", code, out)
	}
}

func TestRunMissingFile(t *testing.T) {
	code, out, errOut := run3(t, []string{td("nope_does_not_exist.txt")}, "")
	if code != 1 {
		t.Errorf("want exit 1 got %d", code)
	}
	if out != "" {
		t.Errorf("want no stdout, got %q", out)
	}
	if !strings.Contains(errOut, "cannot open") {
		t.Errorf("stderr missing diagnostic: %q", errOut)
	}
}

func TestRunMissingFileContinues(t *testing.T) {
	// A missing file should not stop processing of subsequent valid files.
	code, out, errOut := run3(t, []string{"-q", "-n", "1", td("nope.txt"), td("short.txt")}, "")
	if code != 1 {
		t.Errorf("want exit 1 got %d", code)
	}
	if out != "alpha\n" {
		t.Errorf("want valid file output, got %q", out)
	}
	if errOut == "" {
		t.Error("expected stderr diagnostic")
	}
}

func TestRunDirectory(t *testing.T) {
	code, _, errOut := run3(t, []string{"testdata"}, "")
	if code != 1 || !strings.Contains(errOut, "Is a directory") {
		t.Errorf("code=%d err=%q", code, errOut)
	}
}

func TestRunBadFlagExit2(t *testing.T) {
	code, _, errOut := run3(t, []string{"-z"}, "")
	if code != 2 || errOut == "" {
		t.Errorf("code=%d err=%q", code, errOut)
	}
}

func TestRunBadCountExit2(t *testing.T) {
	code, _, errOut := run3(t, []string{"-n", "abc", td("short.txt")}, "")
	if code != 2 || !strings.Contains(errOut, "invalid number") {
		t.Errorf("code=%d err=%q", code, errOut)
	}
}

func TestRunHelp(t *testing.T) {
	code, out, _ := run3(t, []string{"--help"}, "")
	if code != 0 || !strings.Contains(out, "Usage: cchead") {
		t.Errorf("code=%d out=%q", code, out)
	}
}

func TestRunVersion(t *testing.T) {
	code, out, _ := run3(t, []string{"--version"}, "")
	if code != 0 || !strings.Contains(out, "cchead") {
		t.Errorf("code=%d out=%q", code, out)
	}
}

func TestRunDoubleDash(t *testing.T) {
	// Create a file literally named "-n" inside a temp dir to prove -- works.
	dir := t.TempDir()
	p := filepath.Join(dir, "weird.txt")
	if err := os.WriteFile(p, []byte("z\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ := run3(t, []string{"--", p}, "")
	if code != 0 || out != "z\n" {
		t.Errorf("code=%d out=%q", code, out)
	}
}

func TestRunBinaryBytes(t *testing.T) {
	code, out, _ := run3(t, []string{"-c", "5", td("binary.bin")}, "")
	want := "\x00\x01\x02\x03\x04"
	if code != 0 || out != want {
		t.Errorf("code=%d out=%q", code, out)
	}
}

func TestRunCRLF(t *testing.T) {
	code, out, _ := run3(t, []string{"-n", "1", td("crlf.txt")}, "")
	if code != 0 || out != "A\r\n" {
		t.Errorf("code=%d out=%q", code, out)
	}
}
