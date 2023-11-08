package main

import (
	"bytes"
	"strings"
	"testing"
)

func exec(t *testing.T, args []string, input string) (string, string, int) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, strings.NewReader(input), &out, &errb)
	return out.String(), errb.String(), code
}

func TestCLIBasic(t *testing.T) {
	cases := []struct {
		name  string
		args  []string
		input string
		want  string
		code  int
	}{
		{"default_identity", nil, `{"a":1}`, "{\n  \"a\": 1\n}\n", 0},
		{"explicit_identity", []string{"."}, "[1,2]", "[\n  1,\n  2\n]\n", 0},
		{"compact", []string{"-c", "."}, "[1,2,3]", "[1,2,3]\n", 0},
		{"raw", []string{"-r", ".name"}, `{"name":"ccjq"}`, "ccjq\n", 0},
		{"join", []string{"-j", ".[]"}, `["a","b"]`, "ab", 0},
		{"field", []string{"-c", ".commit.message"}, `{"commit":{"message":"init"}}`, "\"init\"\n", 0},
		{"index", []string{"-c", ".[0]"}, "[10,20]", "10\n", 0},
		{"null_input", []string{"-n", "-c", "1+2"}, "ignored", "3\n", 0},
		{"slurp", []string{"-s", "-c", "."}, "1 2 3", "[1,2,3]\n", 0},
		{"stream", []string{"-c", "."}, "1 2 3", "1\n2\n3\n", 0},
		{"compact_cluster", []string{"-cr", ".[]"}, `["a","b"]`, "a\nb\n", 0},
		{"sort_keys", []string{"-cS", "."}, `{"b":1,"a":2}`, `{"a":2,"b":1}` + "\n", 0},
		{"tab_indent", []string{"--tab", "."}, `{"a":1}`, "{\n\t\"a\": 1\n}\n", 0},
		{"indent_n", []string{"--indent", "4", "."}, `{"a":1}`, "{\n    \"a\": 1\n}\n", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, errb, code := exec(t, c.args, c.input)
			if code != c.code {
				t.Errorf("exit code = %d, want %d (stderr: %s)", code, c.code, errb)
			}
			if out != c.want {
				t.Errorf("output = %q, want %q", out, c.want)
			}
		})
	}
}

func TestCLIExitCodes(t *testing.T) {
	t.Run("compile_error", func(t *testing.T) {
		_, errb, code := exec(t, []string{".["}, "[1]")
		if code != 3 {
			t.Errorf("compile error exit = %d, want 3", code)
		}
		if !strings.Contains(errb, "error") {
			t.Errorf("expected error message, got %q", errb)
		}
	})
	t.Run("runtime_error", func(t *testing.T) {
		_, _, code := exec(t, []string{".foo"}, "1")
		if code != 5 {
			t.Errorf("runtime error exit = %d, want 5", code)
		}
	})
	t.Run("exit_status_false", func(t *testing.T) {
		_, _, code := exec(t, []string{"-e", "."}, "false")
		if code != 1 {
			t.Errorf("-e false exit = %d, want 1", code)
		}
	})
	t.Run("exit_status_no_output", func(t *testing.T) {
		_, _, code := exec(t, []string{"-e", ".[]"}, "[]")
		if code != 4 {
			t.Errorf("-e empty exit = %d, want 4", code)
		}
	})
	t.Run("exit_status_truthy", func(t *testing.T) {
		_, _, code := exec(t, []string{"-e", "."}, "42")
		if code != 0 {
			t.Errorf("-e truthy exit = %d, want 0", code)
		}
	})
	t.Run("bad_input", func(t *testing.T) {
		_, _, code := exec(t, []string{"."}, "{not json")
		if code != 2 {
			t.Errorf("bad input exit = %d, want 2", code)
		}
	})
	t.Run("version", func(t *testing.T) {
		_, _, code := exec(t, []string{"--version"}, "")
		if code != 0 {
			t.Errorf("--version exit = %d, want 0", code)
		}
	})
	t.Run("help", func(t *testing.T) {
		_, _, code := exec(t, []string{"-h"}, "")
		if code != 0 {
			t.Errorf("-h exit = %d, want 0", code)
		}
	})
	t.Run("unknown_flag", func(t *testing.T) {
		_, _, code := exec(t, []string{"--bogus"}, "")
		if code != 2 {
			t.Errorf("unknown flag exit = %d, want 2", code)
		}
	})
}

// TestCLIPartialOutput ensures values before a bad input are still flushed.
func TestCLIPartialOutput(t *testing.T) {
	out, _, code := exec(t, []string{"-c", "."}, "1 2 {bad")
	if !strings.HasPrefix(out, "1\n2\n") {
		t.Errorf("expected prior values flushed, got %q", out)
	}
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}
