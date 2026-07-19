package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePrefix(t *testing.T) {
	cases := map[string]byte{
		"C-b": 0x02,
		"C-a": 0x01,
		"^a":  0x01,
		"b":   0x02,
		"C-Z": 0x1a,
	}
	for in, want := range cases {
		got, err := ParsePrefix(in)
		if err != nil {
			t.Fatalf("ParsePrefix(%q): %v", in, err)
		}
		if got != want {
			t.Fatalf("ParsePrefix(%q) = %#x, want %#x", in, got, want)
		}
	}
	for _, bad := range []string{"", "C-1", "Alt-a", "C-ab"} {
		if _, err := ParsePrefix(bad); err == nil {
			t.Fatalf("ParsePrefix(%q) should error", bad)
		}
	}
}

func TestLoadConfigFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cctmux.conf")
	content := "# my config\nset prefix C-a\n\nset-option prefix C-x\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := defaultConfig()
	if err := LoadConfigFile(path, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Prefix != 0x18 { // last wins: C-x
		t.Fatalf("prefix = %#x, want 0x18", cfg.Prefix)
	}
}

func TestLoadConfigMissingIsOK(t *testing.T) {
	cfg := defaultConfig()
	if err := LoadConfigFile(filepath.Join(t.TempDir(), "nope.conf"), &cfg); err != nil {
		t.Fatalf("missing config should be fine: %v", err)
	}
	if cfg.Prefix != 0x02 {
		t.Fatalf("default prefix changed: %#x", cfg.Prefix)
	}
}
