package urls

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSingleProducesNIdentical(t *testing.T) {
	p := Single("http://example.com", 5)
	if p.N() != 5 {
		t.Fatalf("N = %d, want 5", p.N())
	}
	got := p.All()
	want := []string{
		"http://example.com", "http://example.com", "http://example.com",
		"http://example.com", "http://example.com",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("All() = %v, want %v", got, want)
	}
}

func TestFromFileCycles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "urls.txt")
	content := "# comment line\n" +
		"http://a.com\n" +
		"\n" + // blank line
		"   # indented comment\n" +
		"http://b.com\n" +
		"http://c.com\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := FromFile(path, 7)
	if err != nil {
		t.Fatalf("FromFile: %v", err)
	}
	got := p.All()
	want := []string{
		"http://a.com", "http://b.com", "http://c.com",
		"http://a.com", "http://b.com", "http://c.com",
		"http://a.com",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("All() = %v, want cycled %v", got, want)
	}
}

func TestFromFileEmptyIsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(path, []byte("# only a comment\n\n   \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := FromFile(path, 3); err == nil {
		t.Errorf("expected error for empty url file, got nil")
	}
}

func TestFromFileMissing(t *testing.T) {
	if _, err := FromFile(filepath.Join(t.TempDir(), "nope.txt"), 3); err == nil {
		t.Errorf("expected error for missing file, got nil")
	}
}

func TestAt(t *testing.T) {
	p := Single("http://x", 3)
	for i := 0; i < 3; i++ {
		if p.At(i) != "http://x" {
			t.Errorf("At(%d) = %q, want http://x", i, p.At(i))
		}
	}
}
