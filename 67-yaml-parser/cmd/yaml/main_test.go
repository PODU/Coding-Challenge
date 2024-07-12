package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runWith(t *testing.T, args []string, stdin string) (int, string, string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code := run(args, strings.NewReader(stdin), &out, &errBuf)
	return code, out.String(), errBuf.String()
}

func TestValidateValid(t *testing.T) {
	code, out, _ := runWith(t, nil, "name: value\n")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if strings.TrimSpace(out) != "OK" {
		t.Errorf("out = %q, want OK", out)
	}
}

func TestValidateInvalid(t *testing.T) {
	code, _, errOut := runWith(t, nil, "key: \"unterminated\n")
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(errOut, "error") {
		t.Errorf("stderr = %q, want an error message", errOut)
	}
}

func TestJSONStdin(t *testing.T) {
	code, out, _ := runWith(t, []string{"-json", "-"}, "a: 1\nb: 2\n")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, `"a": 1`) || !strings.Contains(out, `"b": 2`) {
		t.Errorf("unexpected json: %s", out)
	}
}

func TestJSONCompact(t *testing.T) {
	code, out, _ := runWith(t, []string{"-json", "-indent", "0", "-"}, "a: 1\n")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if strings.TrimSpace(out) != `{"a":1}` {
		t.Errorf("compact json = %q", out)
	}
}

func TestNDJSON(t *testing.T) {
	code, out, _ := runWith(t, []string{"-ndjson", "-"}, "---\na: 1\n---\nb: 2\n")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 || lines[0] != `{"a":1}` || lines[1] != `{"b":2}` {
		t.Errorf("ndjson lines = %v", lines)
	}
}

func TestStrictRejectsDirective(t *testing.T) {
	code, _, _ := runWith(t, []string{"-strict", "-"}, "%YAML 1.2\n---\na: 1\n")
	if code != 1 {
		t.Fatalf("strict exit = %d, want 1", code)
	}
	// Without -strict the directive is tolerated.
	code, _, _ = runWith(t, []string{"-"}, "%YAML 1.2\n---\na: 1\n")
	if code != 0 {
		t.Fatalf("non-strict exit = %d, want 0", code)
	}
}

func TestMissingFile(t *testing.T) {
	code, _, _ := runWith(t, []string{filepath.Join(t.TempDir(), "nope.yaml")}, "")
	if code != 2 {
		t.Fatalf("missing file exit = %d, want 2", code)
	}
}

func TestFileArg(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "in.yaml")
	if err := os.WriteFile(path, []byte("k: v\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ := runWith(t, []string{"-json", path}, "")
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out, `"k": "v"`) {
		t.Errorf("json = %s", out)
	}
}
