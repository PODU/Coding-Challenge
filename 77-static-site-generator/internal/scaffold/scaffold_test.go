package scaffold

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}

func TestNewSite(t *testing.T) {
	dir := t.TempDir()
	site := filepath.Join(dir, "mysite")
	if err := NewSite(site); err != nil {
		t.Fatalf("NewSite: %v", err)
	}

	for _, rel := range []string{
		filepath.Join("content", "index.md"),
		filepath.Join("themes", "default", "index.html"),
		"config",
	} {
		if _, err := os.Stat(filepath.Join(site, rel)); err != nil {
			t.Errorf("expected %s to exist: %v", rel, err)
		}
	}

	if got := read(t, filepath.Join(site, "config")); strings.TrimSpace(got) != "theme = default" {
		t.Errorf("config = %q", got)
	}
	theme := read(t, filepath.Join(site, "themes", "default", "index.html"))
	if !strings.Contains(theme, "{{ Title }}") || !strings.Contains(theme, "{{ Content }}") {
		t.Errorf("default theme missing required tags")
	}
}

func TestNewSiteExistingNonEmpty(t *testing.T) {
	dir := t.TempDir()
	site := filepath.Join(dir, "mysite")
	if err := os.MkdirAll(site, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(site, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := NewSite(site)
	if !errors.Is(err, ErrExists) {
		t.Fatalf("expected ErrExists, got %v", err)
	}
}

func TestNewSiteIntoEmptyDirAllowed(t *testing.T) {
	dir := t.TempDir() // dir itself is empty
	if err := NewSite(dir); err != nil {
		t.Fatalf("NewSite into empty existing dir: %v", err)
	}
	if !IsSite(dir) {
		t.Errorf("expected %s to be a site", dir)
	}
}

func TestNewTheme(t *testing.T) {
	dir := t.TempDir()
	if err := NewSite(dir); err != nil {
		t.Fatal(err)
	}
	if err := NewTheme(dir, "cc"); err != nil {
		t.Fatalf("NewTheme: %v", err)
	}
	body := read(t, filepath.Join(dir, "themes", "cc", "index.html"))
	if !strings.Contains(body, "{{ Title }}") || !strings.Contains(body, "{{ Content }}") {
		t.Errorf("theme missing required tags")
	}
	if err := NewTheme(dir, "cc"); !errors.Is(err, ErrExists) {
		t.Errorf("expected ErrExists on duplicate theme, got %v", err)
	}
}

func TestNewPage(t *testing.T) {
	dir := t.TempDir()
	if err := NewSite(dir); err != nil {
		t.Fatal(err)
	}
	path, err := NewPage(dir, "about")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}
	if path != filepath.Join(dir, "content", "about.md") {
		t.Errorf("unexpected path %s", path)
	}
	body := read(t, path)
	if !strings.Contains(body, "# About") || !strings.Contains(body, "Write your content here.") {
		t.Errorf("page seed wrong: %q", body)
	}

	if _, err := NewPage(dir, "about"); !errors.Is(err, ErrExists) {
		t.Errorf("expected ErrExists on duplicate page, got %v", err)
	}
}

func TestNewPageNested(t *testing.T) {
	dir := t.TempDir()
	if err := NewSite(dir); err != nil {
		t.Fatal(err)
	}
	path, err := NewPage(dir, "blog/first")
	if err != nil {
		t.Fatalf("NewPage nested: %v", err)
	}
	if path != filepath.Join(dir, "content", "blog", "first.md") {
		t.Errorf("unexpected nested path %s", path)
	}
	if !strings.Contains(read(t, path), "# First") {
		t.Errorf("expected title from base name")
	}
}

func TestFindSiteRoot(t *testing.T) {
	dir := t.TempDir()
	if err := NewSite(dir); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(dir, "content", "blog")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := FindSiteRoot(nested)
	if err != nil {
		t.Fatalf("FindSiteRoot: %v", err)
	}
	// Resolve symlinks (macOS/Windows temp dirs) for a stable compare.
	want, _ := filepath.EvalSymlinks(dir)
	got, _ := filepath.EvalSymlinks(root)
	if got != want {
		t.Errorf("FindSiteRoot = %s, want %s", got, want)
	}
}

func TestFindSiteRootOutsideSite(t *testing.T) {
	dir := t.TempDir()
	if _, err := FindSiteRoot(dir); err == nil {
		t.Error("expected error outside a site")
	}
}
