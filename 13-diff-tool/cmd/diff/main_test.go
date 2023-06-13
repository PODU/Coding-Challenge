package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile creates a temp file with the given content and returns its path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func TestRunGolden(t *testing.T) {
	dir := t.TempDir()

	original := "apple\nbanana\ncherry\ndate\n"
	added := "apple\nbanana\nblueberry\ncherry\ndate\n" // one line added
	removed := "apple\ncherry\ndate\n"                  // banana removed
	changed := "apple\nBANANA\ncherry\ndate\n"          // banana -> BANANA
	empty := ""                                         // empty file
	crlf := "apple\r\nbanana\r\ncherry\r\ndate\r\n"     // same content, CRLF

	files := map[string]string{
		"original": writeFile(t, dir, "original.txt", original),
		"added":    writeFile(t, dir, "added.txt", added),
		"removed":  writeFile(t, dir, "removed.txt", removed),
		"changed":  writeFile(t, dir, "changed.txt", changed),
		"empty":    writeFile(t, dir, "empty.txt", empty),
		"crlf":     writeFile(t, dir, "crlf.txt", crlf),
	}

	cases := []struct {
		name     string
		file1    string
		file2    string
		wantOut  string
		wantCode int
	}{
		{
			name:     "identical files",
			file1:    "original",
			file2:    "original",
			wantOut:  "",
			wantCode: 0,
		},
		{
			name:     "added line",
			file1:    "original",
			file2:    "added",
			wantOut:  "> blueberry\n",
			wantCode: 1,
		},
		{
			name:     "removed line",
			file1:    "original",
			file2:    "removed",
			wantOut:  "< banana\n",
			wantCode: 1,
		},
		{
			name:     "changed line",
			file1:    "original",
			file2:    "changed",
			wantOut:  "< banana\n> BANANA\n",
			wantCode: 1,
		},
		{
			name:     "empty vs non-empty",
			file1:    "empty",
			file2:    "original",
			wantOut:  "> apple\n> banana\n> cherry\n> date\n",
			wantCode: 1,
		},
		{
			name:     "non-empty vs empty",
			file1:    "original",
			file2:    "empty",
			wantOut:  "< apple\n< banana\n< cherry\n< date\n",
			wantCode: 1,
		},
		{
			name:     "both empty",
			file1:    "empty",
			file2:    "empty",
			wantOut:  "",
			wantCode: 0,
		},
		{
			name:     "crlf vs lf are equal",
			file1:    "original",
			file2:    "crlf",
			wantOut:  "",
			wantCode: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run([]string{files[tc.file1], files[tc.file2]}, &stdout, &stderr)
			if code != tc.wantCode {
				t.Errorf("exit code = %d, want %d (stderr: %s)", code, tc.wantCode, stderr.String())
			}
			if stdout.String() != tc.wantOut {
				t.Errorf("stdout = %q, want %q", stdout.String(), tc.wantOut)
			}

			// The --myers flag must yield identical output.
			stdout.Reset()
			stderr.Reset()
			codeM := run([]string{"--myers", files[tc.file1], files[tc.file2]}, &stdout, &stderr)
			if codeM != tc.wantCode || stdout.String() != tc.wantOut {
				t.Errorf("--myers: code=%d out=%q, want code=%d out=%q",
					codeM, stdout.String(), tc.wantCode, tc.wantOut)
			}
		})
	}
}

func TestRunErrors(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{filepath.Join(t.TempDir(), "nope.txt"), filepath.Join(t.TempDir(), "nope2.txt")}, &stdout, &stderr)
		if code != 2 {
			t.Errorf("exit code = %d, want 2", code)
		}
		if stderr.Len() == 0 {
			t.Error("expected an error message on stderr")
		}
	})

	t.Run("too few args", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"only-one"}, &stdout, &stderr)
		if code != 2 {
			t.Errorf("exit code = %d, want 2", code)
		}
		if !strings.Contains(stderr.String(), "usage") {
			t.Errorf("expected usage on stderr, got %q", stderr.String())
		}
	})

	t.Run("too many args", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"a", "b", "c"}, &stdout, &stderr)
		if code != 2 {
			t.Errorf("exit code = %d, want 2", code)
		}
	})

	t.Run("help exits zero", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"--help"}, &stdout, &stderr)
		if code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
		if !strings.Contains(stdout.String(), "usage") {
			t.Errorf("expected usage on stdout, got %q", stdout.String())
		}
	})
}
