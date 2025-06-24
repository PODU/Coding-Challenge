package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tempTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.go", "package a\n\n// note\nvar x = 1\n")
	write("sub/b.py", "# c\nx = 1\n")
	return root
}

func TestRunFilesList(t *testing.T) {
	root := tempTree(t)
	var out, errb bytes.Buffer
	code := run([]string{"--files", root}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit = %d; want 0 (stderr: %s)", code, errb.String())
	}
	lines := strings.Fields(out.String())
	if len(lines) != 2 {
		t.Fatalf("got %d files; want 2:\n%s", len(lines), out.String())
	}
}

func TestRunJSON(t *testing.T) {
	root := tempTree(t)
	var out, errb bytes.Buffer
	code := run([]string{"--json", root}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit = %d; want 0 (stderr: %s)", code, errb.String())
	}
	var parsed map[string]any
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out.String())
	}
	if _, ok := parsed["total"]; !ok {
		t.Errorf("JSON missing total key:\n%s", out.String())
	}
}

func TestRunTextTable(t *testing.T) {
	root := tempTree(t)
	var out, errb bytes.Buffer
	code := run([]string{root}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit = %d; want 0", code)
	}
	if !strings.Contains(out.String(), "Total") {
		t.Errorf("missing Total row:\n%s", out.String())
	}
}

func TestRunThreadsIndependent(t *testing.T) {
	root := tempTree(t)
	var a, b, eb bytes.Buffer
	if c := run([]string{"--json", "--threads", "1", root}, &a, &eb); c != 0 {
		t.Fatalf("exit = %d", c)
	}
	if c := run([]string{"--json", "--threads", "8", root}, &b, &eb); c != 0 {
		t.Fatalf("exit = %d", c)
	}
	if a.String() != b.String() {
		t.Errorf("output differs by thread count:\n1:%s\n8:%s", a.String(), b.String())
	}
}

func TestRunNonexistentPath(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{filepath.Join(t.TempDir(), "nope")}, &out, &errb)
	if code != 2 {
		t.Errorf("exit = %d; want 2 for missing path", code)
	}
}

func TestRunBadFlag(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"--no-such-flag"}, &out, &errb)
	if code != 1 {
		t.Errorf("exit = %d; want 1 for bad flag", code)
	}
}

func TestRunVersion(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"--version"}, &out, &errb)
	if code != 0 || !strings.Contains(out.String(), "loc") {
		t.Errorf("version exit=%d out=%q", code, out.String())
	}
}
