package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fixtureExecutable creates a regular executable file named base (plus the
// platform-appropriate extension) inside dir and returns the resolved path and
// the PATHEXT string to pass to run.
func fixtureExecutable(t *testing.T, dir, base string) (path, pathext string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		// Lowercase ext keeps the constructed candidate path identical to the
		// on-disk filename for exact stdout assertions (match rule itself is
		// case-insensitive; covered in the resolver tests).
		path = filepath.Join(dir, base+".exe")
		pathext = ".com;.exe;.bat"
	} else {
		path = filepath.Join(dir, base)
		pathext = ""
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
	return path, pathext
}

func TestRunUsageNoArgs(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run(nil, &out, &errBuf, "", "")
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if !strings.Contains(errBuf.String(), "usage:") {
		t.Errorf("stderr = %q, want usage line", errBuf.String())
	}
}

func TestRunUnknownFlag(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run([]string{"-z", "foo"}, &out, &errBuf, "", "")
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}

func TestRunFoundSingle(t *testing.T) {
	dir := t.TempDir()
	path, pathext := fixtureExecutable(t, dir, "mytool")

	var out, errBuf bytes.Buffer
	code := run([]string{"mytool"}, &out, &errBuf, dir, pathext)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr=%q)", code, errBuf.String())
	}
	if got := strings.TrimSpace(out.String()); got != path {
		t.Errorf("stdout = %q, want %q", got, path)
	}
}

func TestRunNotFound(t *testing.T) {
	dir := t.TempDir() // empty dir => nothing to find
	var out, errBuf bytes.Buffer
	code := run([]string{"ghosttool"}, &out, &errBuf, dir, ".EXE")
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}
	if !strings.Contains(errBuf.String(), "no ghosttool") {
		t.Errorf("stderr = %q, want not-found diagnostic", errBuf.String())
	}
}

func TestRunMultipleNamesAggregateExit(t *testing.T) {
	dir := t.TempDir()
	_, pathext := fixtureExecutable(t, dir, "mytool")

	var out, errBuf bytes.Buffer
	// One found, one missing => exit 1.
	code := run([]string{"mytool", "ghost"}, &out, &errBuf, dir, pathext)
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "mytool") {
		t.Errorf("stdout = %q, want the found tool", out.String())
	}
}

func TestRunSilentNotFound(t *testing.T) {
	dir := t.TempDir()
	var out, errBuf bytes.Buffer
	code := run([]string{"-s", "ghost"}, &out, &errBuf, dir, ".EXE")
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if out.Len() != 0 || errBuf.Len() != 0 {
		t.Errorf("silent produced output: stdout=%q stderr=%q", out.String(), errBuf.String())
	}
}

func TestRunSilentFound(t *testing.T) {
	dir := t.TempDir()
	_, pathext := fixtureExecutable(t, dir, "mytool")

	var out, errBuf bytes.Buffer
	code := run([]string{"-s", "mytool"}, &out, &errBuf, dir, pathext)
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if out.Len() != 0 {
		t.Errorf("silent produced stdout: %q", out.String())
	}
}

func TestRunAllMatches(t *testing.T) {
	d1 := t.TempDir()
	d2 := t.TempDir()
	p1, pathext := fixtureExecutable(t, d1, "mytool")
	p2, _ := fixtureExecutable(t, d2, "mytool")

	pathEnv := d1 + string(os.PathListSeparator) + d2
	var out, errBuf bytes.Buffer
	code := run([]string{"-a", "mytool"}, &out, &errBuf, pathEnv, pathext)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr=%q)", code, errBuf.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 || lines[0] != p1 || lines[1] != p2 {
		t.Errorf("stdout lines = %v, want [%q %q]", lines, p1, p2)
	}
}
