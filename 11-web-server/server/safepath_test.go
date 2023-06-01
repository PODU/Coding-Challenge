package server

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestResolve_Allowed(t *testing.T) {
	root := mustAbs(t, "testroot")

	cases := map[string]string{
		"root index":      "/",
		"file":            "/a/b.html",
		"dot segment":     "/./a.html",
		"encoded space":   "/my%20file.txt",
		"nested traverse": "/a/../b.html", // stays inside root
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := Resolve(root, in)
			if err != nil {
				t.Fatalf("Resolve(%q) error: %v", in, err)
			}
			if !contained(root, got) {
				t.Fatalf("resolved path %q escaped root %q", got, root)
			}
		})
	}
}

func TestResolve_Forbidden(t *testing.T) {
	root := mustAbs(t, "testroot")

	cases := map[string]string{
		"parent traversal":   "/../etc/passwd",
		"deep traversal":     "/../../../../etc/passwd",
		"encoded traversal":  "/%2e%2e/%2e%2e/etc/passwd",
		"encoded slash dots": "/..%2fetc/passwd",
		"backslash":          "/..\\windows",
		"no leading slash":   "index.html",
		"null byte":          "/a\x00b",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Resolve(root, in); !errors.Is(err, ErrForbidden) {
				t.Fatalf("Resolve(%q) = %v, want ErrForbidden", in, err)
			}
		})
	}
}

func TestResolve_RootItself(t *testing.T) {
	root := mustAbs(t, "testroot")
	got, err := Resolve(root, "/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != root {
		t.Fatalf("Resolve(\"/\") = %q, want root %q", got, root)
	}
}

func TestResolve_StripsQuery(t *testing.T) {
	root := mustAbs(t, "testroot")
	got, err := Resolve(root, "/a.html?x=1&y=2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(root, "a.html")
	if got != want {
		t.Fatalf("Resolve with query = %q, want %q", got, want)
	}
}

func mustAbs(t *testing.T, p string) string {
	t.Helper()
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(abs)
}
