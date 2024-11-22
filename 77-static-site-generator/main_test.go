package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEndToEnd reproduces the brief's reference flow: new site -> new page ->
// build, and asserts the resulting directory structure.
func TestEndToEnd(t *testing.T) {
	work := t.TempDir()
	t.Chdir(work)

	if err := run([]string{"new", "site", "mysite"}); err != nil {
		t.Fatalf("new site: %v", err)
	}
	t.Chdir(filepath.Join(work, "mysite"))

	if err := run([]string{"new", "theme", "cc"}); err != nil {
		t.Fatalf("new theme: %v", err)
	}
	if err := run([]string{"new", "page", "about"}); err != nil {
		t.Fatalf("new page: %v", err)
	}
	if err := run([]string{"build"}); err != nil {
		t.Fatalf("build: %v", err)
	}

	for _, rel := range []string{
		filepath.Join("content", "index.md"),
		filepath.Join("content", "about.md"),
		filepath.Join("themes", "default", "index.html"),
		filepath.Join("themes", "cc", "index.html"),
		filepath.Join("public", "index.html"),
		filepath.Join("public", "about.html"),
	} {
		if _, err := os.Stat(rel); err != nil {
			t.Errorf("expected %s: %v", rel, err)
		}
	}

	about, err := os.ReadFile(filepath.Join("public", "about.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(about), "About") {
		t.Errorf("about.html missing title: %q", string(about))
	}
}

// TestCompatShim verifies `ccsg <name>` aliases `new site <name>`.
func TestCompatShim(t *testing.T) {
	work := t.TempDir()
	t.Chdir(work)
	if err := run([]string{"shimsite"}); err != nil {
		t.Fatalf("shim: %v", err)
	}
	if _, err := os.Stat(filepath.Join("shimsite", "config")); err != nil {
		t.Errorf("shim did not create a site: %v", err)
	}
}

// TestBuildOutsideSite confirms a runtime error (exit code 2) outside a site.
func TestBuildOutsideSite(t *testing.T) {
	t.Chdir(t.TempDir())
	err := run([]string{"build"})
	if err == nil {
		t.Fatal("expected error building outside a site")
	}
	var ce *cliError
	if !asCliError(err, &ce) || ce.code != 2 {
		t.Errorf("expected exit code 2, got %v", err)
	}
}

func asCliError(err error, target **cliError) bool {
	if ce, ok := err.(*cliError); ok {
		*target = ce
		return true
	}
	return false
}
