package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// buildBinary compiles the md5sum binary into a temp dir and returns its path.
func buildBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "md5sum")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}

func TestCLIMultipleFilesAndStdin(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	os.WriteFile(a, []byte("hello"), 0o644)
	os.WriteFile(b, []byte("world"), 0o644)

	// Multiple files plus "-" (stdin) in one invocation.
	cmd := exec.Command(bin, a, "-", b)
	cmd.Stdin = strings.NewReader("abc")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), out)
	}
	if !strings.HasSuffix(lines[1], "  -") || !strings.HasPrefix(lines[1], "900150983cd24fb0d6963f7d28e17f72") {
		t.Errorf("stdin line wrong: %q", lines[1])
	}
}

func TestCLIMissingFileExitCode(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	good := filepath.Join(dir, "g.txt")
	os.WriteFile(good, []byte("hello"), 0o644)

	cmd := exec.Command(bin, filepath.Join(dir, "nope.txt"), good)
	out, err := cmd.Output()
	// Non-zero exit expected.
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected non-zero exit, got err=%v", err)
	}
	if exitErr.ExitCode() != 1 {
		t.Errorf("expected exit 1, got %d", exitErr.ExitCode())
	}
	// The good file should still be hashed on stdout.
	if !strings.Contains(string(out), "5d41402abc4b2a76b9719d911017c592") {
		t.Errorf("good file missing from output: %q", out)
	}
}

func TestCLICheckEndToEnd(t *testing.T) {
	bin := buildBinary(t)
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	os.WriteFile(a, []byte("hello"), 0o644)

	// Generate sums by running the tool, then verify them.
	genCmd := exec.Command(bin, a)
	sums, err := genCmd.Output()
	if err != nil {
		t.Fatalf("gen failed: %v", err)
	}
	cf := filepath.Join(dir, "sums.md5")
	os.WriteFile(cf, sums, 0o644)

	chkCmd := exec.Command(bin, "-c", cf)
	out, err := chkCmd.Output()
	if err != nil {
		t.Fatalf("check failed: %v (out=%q)", err, out)
	}
	if !strings.HasSuffix(strings.TrimSpace(string(out)), ": OK") {
		t.Errorf("expected OK, got %q", out)
	}
}
