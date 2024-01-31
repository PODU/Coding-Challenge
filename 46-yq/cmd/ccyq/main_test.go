package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	quotes  = "../../testdata/quotes.yaml"
	multi   = "../../testdata/multidoc.yaml"
	goldenD = "../../testdata/golden"
)

// runCLI is a helper that invokes run with the given args and stdin.
func runCLI(t *testing.T, args []string, stdin string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = run(args, strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

// TestGolden compares CLI output to checked-in golden files for every feature.
func TestGolden(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		golden string
	}{
		{"identity", []string{".", quotes}, "identity.yaml"},
		{"field", []string{".quotes", quotes}, "quotes.yaml"},
		{"index", []string{".quotes[1]", quotes}, "quote1.yaml"},
		{"chain", []string{".quotes[1].quote", quotes}, "quote1_quote.yaml"},
		{"pipe", []string{".quotes[1] | .quote", quotes}, "quote1_quote.yaml"},
		{"iter", []string{".quotes[]", quotes}, "quotes_iter.yaml"},
		{"iter_field", []string{".quotes[].author", quotes}, "authors.yaml"},
		{"collect", []string{"[.quotes[].quote]", quotes}, "collect_quotes.yaml"},
		{"negative", []string{".quotes[-1]", quotes}, "last_quote.yaml"},
		{"bracket_field", []string{`.["quotes"]`, quotes}, "quotes.yaml"},
		{"leading_bracket", []string{`["quotes"]`, quotes}, "quotes.yaml"},
		{"json_out", []string{"-o", "json", ".", quotes}, "quotes.json"},
		{"json_compact", []string{"-o", "json", "-c", ".quotes[]", quotes}, "quotes_iter_compact.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, out, errs := runCLI(t, tc.args, "")
			if code != 0 {
				t.Fatalf("exit %d, stderr=%s", code, errs)
			}
			want, err := os.ReadFile(filepath.Join(goldenD, tc.golden))
			if err != nil {
				t.Fatal(err)
			}
			if out != string(want) {
				t.Errorf("output mismatch\n--- got ---\n%s\n--- want ---\n%s", out, want)
			}
		})
	}
}

func TestStdinParity(t *testing.T) {
	data, err := os.ReadFile(quotes)
	if err != nil {
		t.Fatal(err)
	}
	codeFile, outFile, _ := runCLI(t, []string{".quotes[0].author", quotes}, "")
	codeStdin, outStdin, _ := runCLI(t, []string{".quotes[0].author"}, string(data))
	if codeFile != 0 || codeStdin != 0 {
		t.Fatalf("exit codes: file=%d stdin=%d", codeFile, codeStdin)
	}
	if outFile != outStdin {
		t.Errorf("stdin/file mismatch: %q vs %q", outStdin, outFile)
	}
	if strings.TrimSpace(outFile) != "Alpha" {
		t.Errorf("unexpected output %q", outFile)
	}
}

func TestMultiDoc(t *testing.T) {
	code, out, _ := runCLI(t, []string{".value", multi}, "")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	want := "1\n---\n2\n---\n3\n"
	if out != want {
		t.Errorf("got %q, want %q", out, want)
	}

	// --no-doc-sep drops separators.
	code, out, _ = runCLI(t, []string{"--no-doc-sep", ".value", multi}, "")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if strings.Contains(out, "---") {
		t.Errorf("--no-doc-sep still has separators: %q", out)
	}
}

func TestRawFlag(t *testing.T) {
	code, out, _ := runCLI(t, []string{"-r", ".quotes[0].quote", quotes}, "")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if strings.Contains(out, `"`) {
		t.Errorf("raw output should have no quotes: %q", out)
	}
	if strings.TrimSpace(out) != "The quick brown fox jumps over the lazy dog." {
		t.Errorf("unexpected raw output %q", out)
	}
}

func TestInPlace(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "m.yaml")
	src, _ := os.ReadFile(multi)
	if err := os.WriteFile(f, src, 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ := runCLI(t, []string{"-i", ".value", f}, "")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if out != "" {
		t.Errorf("in-place should not write stdout, got %q", out)
	}
	got, _ := os.ReadFile(f)
	if string(got) != "1\n---\n2\n---\n3\n" {
		t.Errorf("file content = %q", got)
	}
}

func TestExitCodes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		code int
	}{
		{"success", []string{".", quotes}, 0},
		{"eval_error_missing_key", []string{".nope", quotes}, 1},
		{"bad_filter", []string{".[", quotes}, 2},
		{"inplace_no_file", []string{"-i", "."}, 2},
		{"bad_format", []string{"-o", "xml", ".", quotes}, 2},
		{"missing_file", []string{".", "../../testdata/does-not-exist.yaml"}, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdin := ""
			code, _, _ := runCLI(t, tt.args, stdin)
			if code != tt.code {
				t.Errorf("args %v: exit %d, want %d", tt.args, code, tt.code)
			}
		})
	}
}

func TestOptionalEmptyExitZero(t *testing.T) {
	code, out, _ := runCLI(t, []string{".missing?", quotes}, "")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if out != "" {
		t.Errorf("expected empty output, got %q", out)
	}
}

func TestJSONInputToYAML(t *testing.T) {
	code, out, errs := runCLI(t, []string{"-p", "json", "-o", "yaml", "."}, `{"a":1,"b":[2,3]}`)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, errs)
	}
	if !strings.Contains(out, "a: 1") || !strings.Contains(out, "- 2") {
		t.Errorf("unexpected yaml: %q", out)
	}
}

func TestHelpAndVersion(t *testing.T) {
	if code, _, _ := runCLI(t, []string{"-h"}, ""); code != 0 {
		t.Errorf("help exit %d", code)
	}
	code, _, errs := runCLI(t, []string{"-v"}, "")
	if code != 0 || !strings.Contains(errs, "ccyq") {
		t.Errorf("version: code=%d out=%q", code, errs)
	}
}
