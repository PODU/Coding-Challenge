package builder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ccsg/internal/scaffold"
)

// makeSite builds a minimal site with the given content files and a theme.
func makeSite(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if err := scaffold.NewSite(root); err != nil {
		t.Fatal(err)
	}
	// Replace default index with a known theme using both tags.
	theme := "<html><head><title>{{ Title }}</title></head><body>{{ Content }}</body></html>\n"
	if err := os.WriteFile(filepath.Join(root, "themes", "default", "index.html"), []byte(theme), 0o644); err != nil {
		t.Fatal(err)
	}
	// Remove the scaffolded homepage so only our files exist.
	_ = os.Remove(filepath.Join(root, "content", "index.md"))
	for rel, body := range files {
		p := filepath.Join(root, "content", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}

func TestBuildGolden(t *testing.T) {
	root := makeSite(t, map[string]string{
		"index.md":     "# Home\n\nWelcome to **home**.",
		"about.md":     "# About Us\n\nWe build sites.",
		"blog/post.md": "# First Post\n\nHello blog.",
	})

	res, err := Build(root)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if res.Theme != "default" {
		t.Errorf("theme = %q", res.Theme)
	}
	if len(res.Pages) != 3 {
		t.Errorf("expected 3 pages, got %v", res.Pages)
	}

	index := read(t, filepath.Join(root, "public", "index.html"))
	if !strings.Contains(index, "<title>Home</title>") {
		t.Errorf("index title missing: %q", index)
	}
	if !strings.Contains(index, "<strong>home</strong>") {
		t.Errorf("index content missing: %q", index)
	}

	about := read(t, filepath.Join(root, "public", "about.html"))
	if !strings.Contains(about, "<title>About Us</title>") {
		t.Errorf("about title missing: %q", about)
	}

	// Nested content mirrors into public/blog/post.html.
	post := read(t, filepath.Join(root, "public", "blog", "post.html"))
	if !strings.Contains(post, "<title>First Post</title>") {
		t.Errorf("nested page title missing: %q", post)
	}
}

func TestBuildTitleFallback(t *testing.T) {
	root := makeSite(t, map[string]string{
		"my-page.md": "No heading here, just text.",
	})
	if _, err := Build(root); err != nil {
		t.Fatal(err)
	}
	out := read(t, filepath.Join(root, "public", "my-page.html"))
	if !strings.Contains(out, "<title>My Page</title>") {
		t.Errorf("expected title fallback from filename, got %q", out)
	}
}

func TestBuildEmptyFile(t *testing.T) {
	root := makeSite(t, map[string]string{"empty.md": ""})
	if _, err := Build(root); err != nil {
		t.Fatal(err)
	}
	out := read(t, filepath.Join(root, "public", "empty.html"))
	if !strings.Contains(out, "<title>Empty</title>") {
		t.Errorf("expected filename title for empty file, got %q", out)
	}
}

func TestBuildIgnoresNonMarkdown(t *testing.T) {
	root := makeSite(t, map[string]string{
		"index.md":  "# Home\n",
		"notes.txt": "ignore me",
		"image.png": "binary-ish",
	})
	if _, err := Build(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "public", "notes.html")); err == nil {
		t.Error("non-markdown file should not produce HTML")
	}
}

func TestBuildIdempotentAndCleansStale(t *testing.T) {
	root := makeSite(t, map[string]string{
		"index.md": "# Home\n",
		"gone.md":  "# Gone\n",
	})
	if _, err := Build(root); err != nil {
		t.Fatal(err)
	}
	first := read(t, filepath.Join(root, "public", "index.html"))

	// Second build is byte-identical for unchanged content.
	if _, err := Build(root); err != nil {
		t.Fatal(err)
	}
	if second := read(t, filepath.Join(root, "public", "index.html")); second != first {
		t.Error("build not idempotent")
	}

	// Remove a content file; its stale HTML must disappear after rebuild.
	if err := os.Remove(filepath.Join(root, "content", "gone.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "public", "gone.html")); err == nil {
		t.Error("stale page survived rebuild")
	}
}

func TestActiveTheme(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "config"), []byte("theme = custom\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ActiveTheme(root); got != "custom" {
		t.Errorf("ActiveTheme = %q, want custom", got)
	}
	// Missing config defaults to "default".
	if got := ActiveTheme(t.TempDir()); got != "default" {
		t.Errorf("ActiveTheme default = %q", got)
	}
}

func TestBuildMissingContentTagWarn(t *testing.T) {
	root := makeSite(t, map[string]string{"index.md": "# Home\n"})
	theme := "<html><title>{{ Title }}</title></html>\n"
	if err := os.WriteFile(filepath.Join(root, "themes", "default", "index.html"), []byte(theme), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Build(root)
	if err != nil {
		t.Fatal(err)
	}
	if !res.MissingContentTag {
		t.Error("expected MissingContentTag to be true")
	}
}
