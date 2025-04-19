package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCLI(t *testing.T, args []string, stdin string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

func TestStdinFilter(t *testing.T) {
	code, out, errb := runCLI(t, []string{"--lang", "python"}, "x = 1  # c\n")
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, errb)
	}
	if out != "x = 1\n" {
		t.Errorf("got %q", out)
	}
}

func TestStdinNeedsLang(t *testing.T) {
	code, _, _ := runCLI(t, nil, "x = 1\n")
	if code != 2 {
		t.Errorf("want exit 2, got %d", code)
	}
}

func TestExtensionInference(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.go")
	if err := os.WriteFile(p, []byte("// Doc.\nfunc F() {}\nvar x int // gone\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, errb := runCLI(t, []string{p}, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, errb)
	}
	if !strings.Contains(out, "// Doc.") || strings.Contains(out, "// gone") {
		t.Errorf("doc/pragma handling wrong: %q", out)
	}
}

func TestUnknownExtension(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.xyz")
	os.WriteFile(p, []byte("x"), 0o644)
	code, _, _ := runCLI(t, []string{p}, "")
	if code != 2 {
		t.Errorf("want exit 2 for unknown ext, got %d", code)
	}
}

func TestWriteInPlace(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.py")
	os.WriteFile(p, []byte("x = 1  # c\n"), 0o644)
	code, _, errb := runCLI(t, []string{"-w", p}, "")
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, errb)
	}
	b, _ := os.ReadFile(p)
	if string(b) != "x = 1\n" {
		t.Errorf("in-place got %q", b)
	}
}

func TestCheckMode(t *testing.T) {
	code, _, _ := runCLI(t, []string{"--check", "--lang", "python"}, "x = 1  # c\n")
	if code != 1 {
		t.Errorf("check with comment: want 1, got %d", code)
	}
	code, _, _ = runCLI(t, []string{"--check", "--lang", "python"}, "x = 1\n")
	if code != 0 {
		t.Errorf("check clean: want 0, got %d", code)
	}
}

func TestUnterminatedExit(t *testing.T) {
	code, _, errb := runCLI(t, []string{"--lang", "c"}, "int x; /* open")
	if code != 2 {
		t.Errorf("want exit 2, got %d (stderr=%s)", code, errb)
	}
}

func TestStats(t *testing.T) {
	code, _, errb := runCLI(t, []string{"--stats", "--lang", "python"}, "x = 1  # c\n")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(errb, "removed=1") {
		t.Errorf("stats missing: %q", errb)
	}
}
