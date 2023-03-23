package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runCLI invokes the tool's core with the given args and stdin, returning
// stdout, stderr, and the exit code.
func runCLI(t *testing.T, args []string, stdin string) (string, string, int) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := run(args, strings.NewReader(stdin), &out, &errOut)
	return out.String(), errOut.String(), code
}

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	return p
}

func TestStdinSingleCount(t *testing.T) {
	// Stdin with single count, no filename column.
	out, _, code := runCLI(t, []string{"-l"}, "a\nb\nc\n")
	if code != exitOK {
		t.Fatalf("exit = %d", code)
	}
	if out != "3\n" {
		t.Errorf("stdout = %q, want %q", out, "3\n")
	}
}

func TestStdinEmpty(t *testing.T) {
	// Stdin closed with no data -> zeros, no filename.
	out, _, code := runCLI(t, []string{"-l"}, "")
	if code != exitOK || out != "0\n" {
		t.Errorf("got out=%q code=%d", out, code)
	}
}

func TestStdinDefaultWidth7(t *testing.T) {
	// Multiple counts from stdin use the fixed width 7.
	out, _, _ := runCLI(t, []string{"-l", "-w"}, "a b\nc\n")
	want := "      2       3\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

func TestDashIsStdin(t *testing.T) {
	// "-" routes to stdin and is labeled "-".
	out, _, code := runCLI(t, []string{"-w", "-"}, "x y z\n")
	if code != exitOK {
		t.Fatalf("exit = %d", code)
	}
	if out != "3 -\n" {
		t.Errorf("stdout = %q, want %q", out, "3 -\n")
	}
}

func TestSingleFileNoPad(t *testing.T) {
	f := writeTemp(t, "a.txt", "hello world\nfoo bar baz\n") // 2 lines, 24 bytes
	out, _, code := runCLI(t, []string{"-c", f}, "")
	if code != exitOK {
		t.Fatalf("exit = %d", code)
	}
	if out != "24 "+f+"\n" {
		t.Errorf("stdout = %q", out)
	}
}

func TestMultipleFilesTotal(t *testing.T) {
	// Per-file rows plus a total row, width sized to total bytes.
	a := writeTemp(t, "a.txt", "hello world\nfoo bar baz\n") // 2 5 24
	b := writeTemp(t, "b.txt", "x\n")                        // 1 1 2
	out, _, code := runCLI(t, []string{a, b}, "")
	if code != exitOK {
		t.Fatalf("exit = %d", code)
	}
	want := strings.Join([]string{
		" 2  5 24 " + a,
		" 1  1  2 " + b,
		" 3  6 26 total",
		"",
	}, "\n")
	if out != want {
		t.Errorf("stdout =\n%q\nwant\n%q", out, want)
	}
}

func TestEmptyFile(t *testing.T) {
	// Empty file -> 0 0 0.
	f := writeTemp(t, "e.txt", "")
	out, _, code := runCLI(t, []string{f}, "")
	if code != exitOK {
		t.Fatalf("exit = %d", code)
	}
	if out != "0 0 0 "+f+"\n" {
		t.Errorf("stdout = %q", out)
	}
}

func TestMissingFile(t *testing.T) {
	// Nonexistent file -> stderr message, exit 1.
	missing := filepath.Join(t.TempDir(), "nope.txt")
	out, errOut, code := runCLI(t, []string{missing}, "")
	if code != exitFileErr {
		t.Fatalf("exit = %d, want %d", code, exitFileErr)
	}
	if out != "" {
		t.Errorf("stdout should be empty, got %q", out)
	}
	want := "ccwc: " + missing + ": No such file or directory\n"
	if errOut != want {
		t.Errorf("stderr = %q, want %q", errOut, want)
	}
}

func TestMissingThenValidFile(t *testing.T) {
	// Continue processing remaining files after an error; total reflects
	// only successful files.
	missing := filepath.Join(t.TempDir(), "nope.txt")
	b := writeTemp(t, "b.txt", "x\n")
	out, errOut, code := runCLI(t, []string{missing, b}, "")
	if code != exitFileErr {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(errOut, "No such file or directory") {
		t.Errorf("stderr = %q", errOut)
	}
	// b.txt row + total row should still be printed.
	if !strings.Contains(out, b) || !strings.Contains(out, "total") {
		t.Errorf("stdout = %q", out)
	}
}

func TestDirectory(t *testing.T) {
	// Directory argument -> error, exit 1.
	dir := t.TempDir()
	out, errOut, code := runCLI(t, []string{dir}, "")
	if code != exitFileErr {
		t.Fatalf("exit = %d, want %d", code, exitFileErr)
	}
	if out != "" {
		t.Errorf("stdout should be empty, got %q", out)
	}
	want := "ccwc: " + dir + ": Is a directory\n"
	if errOut != want {
		t.Errorf("stderr = %q, want %q", errOut, want)
	}
}

func TestUnknownFlag(t *testing.T) {
	// Unknown flag -> usage error, exit 2.
	_, errOut, code := runCLI(t, []string{"-x"}, "")
	if code != exitUsage {
		t.Fatalf("exit = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errOut, "invalid option") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestHelp(t *testing.T) {
	out, _, code := runCLI(t, []string{"-h"}, "")
	if code != exitOK {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out, "Usage: ccwc") {
		t.Errorf("help text missing: %q", out)
	}
}

func TestColumnOrderInvariance(t *testing.T) {
	// Regardless of flag order, columns print lines, words, chars,
	// bytes.
	f := writeTemp(t, "f.txt", "a b\n") // lines 1, words 2, chars 4, bytes 4
	for _, args := range [][]string{
		{"-cl", f},
		{"-lc", f},
		{"-c", "-l", f},
	} {
		out, _, code := runCLI(t, args, "")
		if code != exitOK {
			t.Fatalf("args %v exit %d", args, code)
		}
		// lines (1) then bytes (4): "1 4 f.txt"
		if !strings.HasPrefix(out, "1 4 ") {
			t.Errorf("args %v: stdout = %q, want lines-before-bytes order", args, out)
		}
	}
}

func TestMultibyteCharsVsBytes(t *testing.T) {
	// -m < -c for multibyte content.
	f := writeTemp(t, "u.txt", "héllo\n") // 6 chars, 7 bytes
	mOut, _, _ := runCLI(t, []string{"-m", f}, "")
	cOut, _, _ := runCLI(t, []string{"-c", f}, "")
	if !strings.HasPrefix(mOut, "6 ") {
		t.Errorf("-m stdout = %q, want 6", mOut)
	}
	if !strings.HasPrefix(cOut, "7 ") {
		t.Errorf("-c stdout = %q, want 7", cOut)
	}
}

// Differential tests against the system wc, skipped if it is not installed.

var oracleFixtures = map[string]string{
	"ascii":      "the quick brown fox\njumps over the lazy dog\n",
	"no_newline": "trailing line without newline",
	"utf8":       "héllo wörld\n日本語 テスト\n",
	"crlf":       "a b\r\nc d e\r\n",
	"whitespace": "   \t\n  \n\t\t\n",
	"empty":      "",
	"multiline":  strings.Repeat("lorem ipsum dolor sit amet\n", 50),
}

func TestDifferentialAgainstSystemWC(t *testing.T) {
	wcPath, err := exec.LookPath("wc")
	if err != nil {
		t.Skip("system wc not found; skipping differential tests")
	}

	flagSets := [][]string{
		{}, {"-c"}, {"-l"}, {"-w"}, {"-m"}, {"-l", "-w"}, {"-c", "-l", "-w"},
	}

	dir := t.TempDir()
	// Write fixtures to disk so both tools read identical bytes.
	paths := make(map[string]string)
	for name, content := range oracleFixtures {
		p := filepath.Join(dir, name+".txt")
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		paths[name] = p
	}

	for _, flags := range flagSets {
		for name, p := range paths {
			t.Run(strings.Join(append([]string{name}, flags...), "_"), func(t *testing.T) {
				// Oracle output.
				oracle, err := exec.Command(wcPath, append(append([]string{}, flags...), p)...).Output()
				if err != nil {
					t.Fatalf("system wc failed: %v", err)
				}
				// Our output.
				var out, errOut bytes.Buffer
				code := run(append(append([]string{}, flags...), p), strings.NewReader(""), &out, &errOut)
				if code != exitOK {
					t.Fatalf("ccwc exit %d, stderr=%q", code, errOut.String())
				}
				if out.String() != string(oracle) {
					t.Errorf("flags %v file %s:\n ccwc: %q\n  wc:  %q", flags, name, out.String(), string(oracle))
				}
			})
		}
	}
}

func TestDifferentialMultiFile(t *testing.T) {
	wcPath, err := exec.LookPath("wc")
	if err != nil {
		t.Skip("system wc not found")
	}
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	os.WriteFile(a, []byte("hello world\nfoo bar baz\n"), 0o644)
	os.WriteFile(b, []byte("single\n"), 0o644)

	for _, flags := range [][]string{{}, {"-l"}, {"-c"}, {"-l", "-w", "-c"}} {
		args := append(append([]string{}, flags...), a, b)
		oracle, err := exec.Command(wcPath, args...).Output()
		if err != nil {
			t.Fatalf("wc failed: %v", err)
		}
		var out, errOut bytes.Buffer
		run(args, strings.NewReader(""), &out, &errOut)
		if out.String() != string(oracle) {
			t.Errorf("flags %v:\n ccwc: %q\n  wc:  %q", flags, out.String(), string(oracle))
		}
	}
}

func TestDifferentialStdin(t *testing.T) {
	wcPath, err := exec.LookPath("wc")
	if err != nil {
		t.Skip("system wc not found")
	}
	input := "the quick brown fox\njumps\n"
	for _, flags := range [][]string{{}, {"-l"}, {"-w"}, {"-c"}, {"-l", "-w"}} {
		cmd := exec.Command(wcPath, flags...)
		cmd.Stdin = strings.NewReader(input)
		oracle, err := cmd.Output()
		if err != nil {
			t.Fatalf("wc failed: %v", err)
		}
		var out, errOut bytes.Buffer
		run(flags, strings.NewReader(input), &out, &errOut)
		if out.String() != string(oracle) {
			t.Errorf("stdin flags %v:\n ccwc: %q\n  wc:  %q", flags, out.String(), string(oracle))
		}
	}
}
