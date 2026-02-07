package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

func runArgs(t *testing.T, args []string, stdin string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, &out, &errb, strings.NewReader(stdin))
	return code, out.String(), errb.String()
}

func TestCLIValid(t *testing.T) {
	schema := writeTemp(t, "schema.json", `{"type":"integer","minimum":0}`)
	inst := writeTemp(t, "inst.json", `5`)
	code, out, _ := runArgs(t, []string{"--schema", schema, inst}, "")
	if code != exitValid {
		t.Fatalf("exit = %d, want %d", code, exitValid)
	}
	if strings.TrimSpace(out) != "OK" {
		t.Fatalf("out = %q, want OK", out)
	}
}

func TestCLIInvalid(t *testing.T) {
	schema := writeTemp(t, "schema.json", `{"properties":{"age":{"minimum":18}}}`)
	inst := writeTemp(t, "inst.json", `{"age":5}`)
	code, out, _ := runArgs(t, []string{"--schema", schema, inst}, "")
	if code != exitInvalid {
		t.Fatalf("exit = %d, want %d", code, exitInvalid)
	}
	if !strings.Contains(out, "/age") || !strings.Contains(out, "minimum") {
		t.Fatalf("out missing path/keyword: %q", out)
	}
}

func TestCLIJSONFormat(t *testing.T) {
	schema := writeTemp(t, "schema.json", `{"type":"string"}`)
	inst := writeTemp(t, "inst.json", `5`)
	code, out, _ := runArgs(t, []string{"--schema", schema, "--format", "json", inst}, "")
	if code != exitInvalid {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(out, `"valid": false`) || !strings.Contains(out, `"keyword": "type"`) {
		t.Fatalf("json output unexpected: %q", out)
	}
}

func TestCLIStdin(t *testing.T) {
	schema := writeTemp(t, "schema.json", `{"type":"integer"}`)
	code, out, _ := runArgs(t, []string{"--schema", schema, "-"}, `42`)
	if code != exitValid {
		t.Fatalf("exit = %d, out=%q", code, out)
	}
}

func TestCLIMissingSchema(t *testing.T) {
	inst := writeTemp(t, "inst.json", `1`)
	code, _, errOut := runArgs(t, []string{inst}, "")
	if code != exitUsage {
		t.Fatalf("exit = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(errOut, "schema") {
		t.Fatalf("stderr should mention schema: %q", errOut)
	}
}

func TestCLIBadFormatFlag(t *testing.T) {
	schema := writeTemp(t, "schema.json", `{}`)
	inst := writeTemp(t, "inst.json", `1`)
	code, _, _ := runArgs(t, []string{"--schema", schema, "--format", "xml", inst}, "")
	if code != exitUsage {
		t.Fatalf("exit = %d, want %d", code, exitUsage)
	}
}

func TestCLIMalformedSchema(t *testing.T) {
	schema := writeTemp(t, "schema.json", `{bad`)
	inst := writeTemp(t, "inst.json", `1`)
	code, _, errOut := runArgs(t, []string{"--schema", schema, inst}, "")
	if code != exitInput {
		t.Fatalf("exit = %d, want %d", code, exitInput)
	}
	if !strings.Contains(errOut, "schema") {
		t.Fatalf("stderr should mention schema: %q", errOut)
	}
}

func TestCLIMalformedInstance(t *testing.T) {
	schema := writeTemp(t, "schema.json", `{}`)
	inst := writeTemp(t, "inst.json", `[1,`)
	code, _, _ := runArgs(t, []string{"--schema", schema, inst}, "")
	if code != exitInput {
		t.Fatalf("exit = %d, want %d", code, exitInput)
	}
}

func TestCLIMissingFile(t *testing.T) {
	schema := writeTemp(t, "schema.json", `{}`)
	code, _, _ := runArgs(t, []string{"--schema", schema, "does-not-exist.json"}, "")
	if code != exitInput {
		t.Fatalf("exit = %d, want %d", code, exitInput)
	}
}

func TestCLIMaxErrors(t *testing.T) {
	schema := writeTemp(t, "schema.json", `{"properties":{"a":{"type":"integer"},"b":{"type":"integer"},"c":{"type":"integer"}}}`)
	inst := writeTemp(t, "inst.json", `{"a":"x","b":"y","c":"z"}`)
	code, out, _ := runArgs(t, []string{"--schema", schema, "--max-errors", "1", inst}, "")
	if code != exitInvalid {
		t.Fatalf("exit = %d", code)
	}
	if got := strings.Count(strings.TrimSpace(out), "\n") + 1; got != 1 {
		t.Fatalf("expected 1 error line, got %d: %q", got, out)
	}
}
