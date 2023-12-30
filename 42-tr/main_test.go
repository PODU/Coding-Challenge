package main

import (
	"bytes"
	"strings"
	"testing"
)

// runCLI exercises the full CLI path: arg parsing -> SET parsing -> engine.
func runCLI(args []string, input string) (string, int) {
	var out, errBuf bytes.Buffer
	code := run(args, strings.NewReader(input), &out, &errBuf)
	return out.String(), code
}

// TestGolden covers the behavioural matrix, with expected
// outputs matching GNU tr.
func TestGolden(t *testing.T) {
	cases := []struct {
		name  string
		args  []string
		input string
		want  string
	}{
		{"basic translate", []string{"a", "b"}, "aaa", "bbb"},
		{"range upper", []string{"a-z", "A-Z"}, "Hello", "HELLO"},
		{"class case map", []string{"[:lower:]", "[:upper:]"}, "hi!", "HI!"},
		{"class case map reverse", []string{"[:upper:]", "[:lower:]"}, "HI!", "hi!"},
		{"pad short SET2", []string{"abc", "x"}, "cab", "xxx"},
		{"surplus SET2 ignored", []string{"ab", "wxyz"}, "abc", "wxc"},
		{"delete range", []string{"-d", "a-z"}, "He1lo", "H1"},
		{"delete class", []string{"-d", "[:digit:]"}, "a1b2", "ab"},
		{"squeeze space", []string{"-s", " "}, "a   b", "a b"},
		{"squeeze class", []string{"-s", "[:space:]"}, "a\t\t  b", "a\t b"},
		{"translate+squeeze", []string{"-s", "a-z", "A-Z"}, "aabb", "AB"},
		{"complement delete", []string{"-cd", "[:alpha:]"}, "a1b2", "ab"},
		{"word split", []string{"-cs", "[:alpha:]", "\\n"}, "a, b\n", "a\nb\n"},
		{"delete+squeeze", []string{"-ds", "a", "b"}, "aabbb", "b"},
		{"delete+squeeze keeps deleted out", []string{"-ds", "a", "ab"}, "aabbb", "b"},
		{"octal escape", []string{"\\101", "x"}, "A", "x"},
		{"literal dash via dashdash", []string{"-d", "--", "-"}, "a-b", "ab"},
		{"literal dash via escape", []string{"-d", "\\-"}, "a-b", "ab"},
		{"escape newline translate", []string{"\\n", "X"}, "a\nb", "aXb"},
		{"escape tab to space", []string{"\\t", " "}, "a\tb", "a b"},
		{"repeat exact", []string{"ab", "[y*2]"}, "ab", "yy"},
		{"repeat pad", []string{"abc", "[z*]"}, "abc", "zzz"},
		{"truncate set1", []string{"-t", "abc", "xy"}, "abc", "xyc"},
		{"concatenated ranges", []string{"0-9a-f", "X"}, "5e!", "XX!"},
		{"equivalence class", []string{"[=a=]", "X"}, "aba", "XbX"},
		{"complement squeeze translate", []string{"-cs", "[:alpha:]", "x"}, "a..b", "axb"},
		{"duplicate set1 first wins", []string{"aa", "xy"}, "a", "x"},
		{"empty input", []string{"a", "b"}, "", ""},
		{"nul octal", []string{"\\0", "X"}, "a\x00b", "aXb"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, code := runCLI(tc.args, tc.input)
			if code != 0 {
				t.Fatalf("exit code = %d, want 0", code)
			}
			if got != tc.want {
				t.Errorf("args=%v input=%q\n got=%q\nwant=%q", tc.args, tc.input, got, tc.want)
			}
		})
	}
}

func TestErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"missing SET2 translate", []string{"abc"}},
		{"empty SET2 translate", []string{"a", ""}},
		{"illegal class in SET2", []string{"a-z", "[:digit:]"}},
		{"reversed range", []string{"z-a", "x"}},
		{"reversed range delete", []string{"-d", "9-0"}},
		{"unknown class", []string{"-d", "[:bogus:]"}},
		{"extra operand delete", []string{"-d", "a", "b"}},
		{"missing operand", []string{}},
		{"three operands", []string{"a", "b", "c"}},
		{"delete squeeze one set", []string{"-ds", "a"}},
		{"pad in set1", []string{"[a*]", "x"}},
		{"bad option", []string{"-z", "a", "b"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, code := runCLI(tc.args, "input")
			if code != 1 {
				t.Errorf("args=%v exit code = %d, want 1", tc.args, code)
			}
		})
	}
}

func TestHelpVersion(t *testing.T) {
	out, code := runCLI([]string{"--help"}, "")
	if code != 0 || !strings.Contains(out, "Usage: tr") {
		t.Errorf("--help: code=%d out=%q", code, out)
	}
	out, code = runCLI([]string{"--version"}, "")
	if code != 0 || !strings.Contains(out, "1.0.0") {
		t.Errorf("--version: code=%d out=%q", code, out)
	}
}

// TestStreamBoundary ensures squeeze collapses runs that span the internal
// 64 KiB read buffer.
func TestStreamBoundary(t *testing.T) {
	input := strings.Repeat("a", 200000)
	got, code := runCLI([]string{"-s", "a"}, input)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if got != "a" {
		t.Errorf("squeeze across boundary: got len %d, want 1", len(got))
	}
}

func TestComplementTranslate(t *testing.T) {
	// Non-alpha chars all map to '#'.
	got, code := runCLI([]string{"-c", "[:alpha:]", "#"}, "ab,cd!")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if got != "ab#cd#" {
		t.Errorf("got %q, want %q", got, "ab#cd#")
	}
}
