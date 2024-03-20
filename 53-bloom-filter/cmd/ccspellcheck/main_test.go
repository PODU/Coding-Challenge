package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseArgs(t *testing.T) {
	cfg, err := parseArgs([]string{"-build", "d.txt", "-o", "out.bf", "-p", "0.001"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.buildPath != "d.txt" || cfg.outPath != "out.bf" || cfg.p != 0.001 {
		t.Errorf("unexpected cfg: %+v", cfg)
	}

	cfg, err = parseArgs([]string{"-f", "m.bf", "hello", "world"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.modelPath != "m.bf" || len(cfg.words) != 2 {
		t.Errorf("unexpected cfg: %+v", cfg)
	}

	if _, err := parseArgs([]string{"-build"}); err == nil {
		t.Error("expected error for -build without value")
	}
	if _, err := parseArgs([]string{"-bogus"}); err == nil {
		t.Error("expected error for unknown flag")
	}
}

// TestBuildThenCheck exercises a full build -> persist -> check round-trip
// through the run() entry point, asserting exit codes and output.
func TestBuildThenCheck(t *testing.T) {
	dir := t.TempDir()
	dict := filepath.Join(dir, "dict.txt")
	model := filepath.Join(dir, "words.bf")

	// BOM on first line, blank lines, and mixed case to exercise normalization.
	content := "\ufeffapple\nBanana\n\n  cherry  \ndurian\n"
	if err := os.WriteFile(dict, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	code := run([]string{"-build", dict, "-o", model}, &out, &errb)
	if code != exitOK {
		t.Fatalf("build exit = %d, stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "n=4") {
		t.Errorf("build summary missing n=4: %q", out.String())
	}
	if _, err := os.Stat(model); err != nil {
		t.Fatalf("model not written: %v", err)
	}

	// All in-dictionary words correct (case-insensitive) -> exit 0.
	out.Reset()
	errb.Reset()
	code = run([]string{"-f", model, "apple", "BANANA", "cherry"}, &out, &errb)
	if code != exitOK {
		t.Errorf("check all-correct exit = %d, out=%q", code, out.String())
	}

	// A fabricated nonsense word -> flagged, exit 1.
	out.Reset()
	errb.Reset()
	code = run([]string{"-f", model, "apple", "zzqxnonsenseword"}, &out, &errb)
	if code != exitFlagged {
		t.Errorf("check some-misspelled exit = %d, want %d", code, exitFlagged)
	}
	if !strings.Contains(out.String(), "zzqxnonsenseword") {
		t.Errorf("flagged word not reported: %q", out.String())
	}
	if strings.Contains(out.String(), "apple") {
		t.Errorf("correct word should not be flagged: %q", out.String())
	}

	// No words in check mode -> usage error, exit 2.
	out.Reset()
	errb.Reset()
	code = run([]string{"-f", model}, &out, &errb)
	if code != exitUsage {
		t.Errorf("no-words exit = %d, want %d", code, exitUsage)
	}

	// Missing model file -> usage/IO error, exit 2.
	out.Reset()
	errb.Reset()
	code = run([]string{"-f", filepath.Join(dir, "nope.bf"), "apple"}, &out, &errb)
	if code != exitUsage {
		t.Errorf("missing-model exit = %d, want %d", code, exitUsage)
	}
}

func TestHelp(t *testing.T) {
	var out, errb bytes.Buffer
	code := run([]string{"-h"}, &out, &errb)
	if code != exitOK {
		t.Errorf("help exit = %d", code)
	}
	if !strings.Contains(out.String(), "ccspellcheck") {
		t.Errorf("help text missing: %q", out.String())
	}
}
