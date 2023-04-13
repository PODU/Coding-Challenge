package main

import (
	"bytes"
	"strings"
	"testing"
)

// runArgs is a helper that invokes run with the given args and stdin,
// returning stdout, stderr, and the exit code.
func runArgs(t *testing.T, args []string, stdin string) (string, string, int) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code := run(args, strings.NewReader(stdin), &out, &errBuf)
	return out.String(), errBuf.String(), code
}

func TestIntegration_Examples(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		stdin string
		want  string
		code  int
	}{
		{
			name: "cut -f2 sample.tsv",
			args: []string{"-f2", "../../testdata/sample.tsv"},
			want: "f1\n1\n6\n11\n16\n21\n",
		},
		{
			name: "cut -f1,2 -d, fourchords.csv",
			args: []string{"-f1,2", "-d,", "../../testdata/fourchords.csv"},
			want: "Song,Artist\nLet It Be,The Beatles\nNo Woman No Cry,Bob Marley\nDon't Stop Believin',Journey\nSomeone Like You,Adele\nAfrica,Toto\nWith or Without You,U2\n",
		},
		{
			name: "whitespace field list",
			args: []string{"-d,", "-f", "1 2", "../../testdata/fourchords.csv"},
			want: "Song,Artist\nLet It Be,The Beatles\nNo Woman No Cry,Bob Marley\nDon't Stop Believin',Journey\nSomeone Like You,Adele\nAfrica,Toto\nWith or Without You,U2\n",
		},
		{
			name: "byte range",
			args: []string{"-b1-3", "../../testdata/fourchords.csv"},
			want: "Son\nLet\nNo \nDon\nSom\nAfr\nWit\n",
		},
		{
			name:  "stdin via dash",
			args:  []string{"-d-", "-f2", "--output-delimiter=:", "-"},
			stdin: "a-b-c\n",
			want:  "b\n",
		},
		{
			name:  "complement field",
			args:  []string{"-d,", "-f1", "--complement"},
			stdin: "a,b,c\n",
			want:  "b,c\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, errOut, code := runArgs(t, tt.args, tt.stdin)
			if code != tt.code {
				t.Errorf("exit code = %d, want %d (stderr: %s)", code, tt.code, errOut)
			}
			if out != tt.want {
				t.Errorf("stdout = %q, want %q", out, tt.want)
			}
		})
	}
}

func TestIntegration_StdinMatchesFile(t *testing.T) {
	fileOut, _, _ := runArgs(t, []string{"-f2", "../../testdata/sample.tsv"}, "")

	// Feed the same content via stdin.
	content := "f0\tf1\tf2\tf3\tf4\n0\t1\t2\t3\t4\n5\t6\t7\t8\t9\n10\t11\t12\t13\t14\n15\t16\t17\t18\t19\n20\t21\t22\t23\t24\n"
	stdinOut, _, _ := runArgs(t, []string{"-f2"}, content)

	if fileOut != stdinOut {
		t.Errorf("file path %q != stdin path %q", fileOut, stdinOut)
	}
}

func TestIntegration_MissingFile(t *testing.T) {
	out, errOut, code := runArgs(t, []string{"-f1", "../../testdata/sample.tsv", "does-not-exist.tsv"}, "")
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(errOut, "does-not-exist.tsv") || !strings.Contains(errOut, "No such file or directory") {
		t.Errorf("stderr = %q, want mention of missing file", errOut)
	}
	// Output from the good file should still be present.
	if !strings.HasPrefix(out, "f0\n") {
		t.Errorf("stdout = %q, want output from the readable file", out)
	}
}

func TestIntegration_UsageError(t *testing.T) {
	_, errOut, code := runArgs(t, []string{"-f1", "-b1"}, "")
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut, "only one type of list") {
		t.Errorf("stderr = %q", errOut)
	}
}

func TestIntegration_HelpVersion(t *testing.T) {
	out, _, code := runArgs(t, []string{"--help"}, "")
	if code != 0 || !strings.Contains(out, "Usage: cut") {
		t.Errorf("help: code=%d out=%q", code, out)
	}
	out, _, code = runArgs(t, []string{"--version"}, "")
	if code != 0 || !strings.Contains(out, "cut") {
		t.Errorf("version: code=%d out=%q", code, out)
	}
}
