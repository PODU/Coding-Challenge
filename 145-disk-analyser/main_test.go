package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ccdisk/internal/analysis"
	"ccdisk/internal/testtree"
)

func TestParseSizeAndAge(t *testing.T) {
	for in, want := range map[string]int64{"0": 0, "512": 512, "10M": 10 << 20, "1G": 1 << 30, "1.5k": 1536, "2MB": 2 << 20} {
		if got, err := parseSize(in); err != nil || got != want {
			t.Errorf("parseSize(%q) = %d, %v", in, got, err)
		}
	}
	for in, want := range map[string]time.Duration{"0": 0, "30d": 30 * 24 * time.Hour, "6mo": 6 * analysis.Month, "1y": analysis.Year, "2w": 14 * 24 * time.Hour, "12h": 12 * time.Hour} {
		if got, err := parseAge(in); err != nil || got != want {
			t.Errorf("parseAge(%q) = %v, %v", in, got, err)
		}
	}
	for _, bad := range []string{"x", "-1", "10Q"} {
		if _, err := parseSize(bad); err == nil {
			t.Errorf("parseSize(%q) should fail", bad)
		}
	}
	if _, err := parseAge("soon"); err == nil {
		t.Error("parseAge should fail")
	}
}

func TestReport(t *testing.T) {
	root := filepath.Join(t.TempDir(), "testroot")
	if _, err := testtree.Build(root); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := run([]string{"--report", "--min-size", "10M", "--min-age", "1y", root}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	s := out.String()
	for _, want := range []string{
		"50.0 MB", "├─ media  (4 files)",
		"hard link, space counted under another name",
		"Largest files (disk usage, at least 10.0 MB, unmodified for 1y):\n   15.0 MB  2019-01-01  " + filepath.Join("old", "archive.bin") + "\n\n",
		"Reclaimable (34.0 MB total, nothing removed)",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("report missing %q:\n%s", want, s)
		}
	}
}

func TestBadArguments(t *testing.T) {
	var out, errb bytes.Buffer
	if run([]string{"a", "b"}, &out, &errb) == 0 || run([]string{"--min-size", "x", "."}, &out, &errb) == 0 {
		t.Fatal("bad arguments should fail")
	}
	if run([]string{filepath.Join(t.TempDir(), "missing")}, &out, &errb) == 0 {
		t.Fatal("a missing directory should fail")
	}
	f := filepath.Join(t.TempDir(), "file")
	os.WriteFile(f, nil, 0o644)
	if run([]string{"--report", f}, &out, &errb) == 0 {
		t.Fatal("a file is not a directory")
	}
}
