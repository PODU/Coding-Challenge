package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dump.rdb")

	s := New()
	s.Set("name", "claude", SetOptions{}, false)
	s.Push("list", []string{"a", "b", "c"}, false)
	if err := s.Save(path); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	s2 := New()
	if err := s2.Load(path); err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if got, _, _ := s2.Get("name"); got != "claude" {
		t.Errorf("loaded name = %q, want claude", got)
	}
	list, _ := s2.LRange("list", 0, -1)
	if len(list) != 3 || list[0] != "a" || list[2] != "c" {
		t.Errorf("loaded list = %v, want [a b c]", list)
	}
}

func TestLoadMissingFile(t *testing.T) {
	s := New()
	if err := s.Load(filepath.Join(t.TempDir(), "does-not-exist.rdb")); err != nil {
		t.Errorf("Load of missing file should be nil error, got %v", err)
	}
}

func TestLoadCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.rdb")
	if err := os.WriteFile(path, []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := New()
	if err := s.Load(path); err == nil {
		t.Errorf("expected error loading corrupt snapshot")
	}
}
