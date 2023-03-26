package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runWith invokes run() with the given args and stdin, capturing streams.
func runWith(args []string, stdin string) (code int, stdout, stderr string) {
	var out, errb bytes.Buffer
	code = run(args, strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

func TestValidFromStdin(t *testing.T) {
	code, out, errb := runWith(nil, `{"a":1}`)
	if code != 0 {
		t.Fatalf("exit %d, want 0 (stderr=%q)", code, errb)
	}
	if !strings.Contains(out, "Valid JSON") {
		t.Errorf("stdout=%q, want it to contain 'Valid JSON'", out)
	}
}

func TestInvalidFromStdin(t *testing.T) {
	code, _, errb := runWith(nil, `{"a":}`)
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errb, "Invalid JSON") || !strings.Contains(errb, "line") {
		t.Errorf("stderr=%q, want 'Invalid JSON ... line ...'", errb)
	}
}

func TestQuietSuppressesOutput(t *testing.T) {
	code, out, errb := runWith([]string{"-q"}, `{}`)
	if code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	if out != "" || errb != "" {
		t.Errorf("quiet mode produced output: stdout=%q stderr=%q", out, errb)
	}

	code, out, errb = runWith([]string{"--quiet"}, `{`)
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if out != "" || errb != "" {
		t.Errorf("quiet mode produced output on error: stdout=%q stderr=%q", out, errb)
	}
}

func TestVerbose(t *testing.T) {
	code, out, _ := runWith([]string{"-v"}, `[1,2,3]`)
	if code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	if !strings.Contains(out, "array with 3 element") {
		t.Errorf("verbose stdout=%q, want array summary", out)
	}
}

func TestHelpAndVersion(t *testing.T) {
	if code, out, _ := runWith([]string{"--help"}, ""); code != 0 || !strings.Contains(out, "Usage:") {
		t.Errorf("--help: code=%d out=%q", code, out)
	}
	if code, out, _ := runWith([]string{"--version"}, ""); code != 0 || !strings.Contains(out, "jsonparser") {
		t.Errorf("--version: code=%d out=%q", code, out)
	}
}

func TestUnknownFlag(t *testing.T) {
	code, _, _ := runWith([]string{"--bogus"}, "")
	if code != 2 {
		t.Errorf("unknown flag exit %d, want 2", code)
	}
}

func TestFileArg(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.json")
	if err := os.WriteFile(good, []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := runWith([]string{good}, ""); code != 0 {
		t.Errorf("valid file exit %d, want 0", code)
	}

	if code, _, errb := runWith([]string{filepath.Join(dir, "missing.json")}, ""); code != 1 {
		t.Errorf("missing file exit %d, want 1 (stderr=%q)", code, errb)
	}
}
