package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

// mkRepo creates dir and a .git subdirectory inside it.
func mkRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func contains(repos []Repo, path string) bool {
	abs, _ := filepath.Abs(path)
	for _, r := range repos {
		if r.Path == abs {
			return true
		}
	}
	return false
}

func TestFindReposAndGitFile(t *testing.T) {
	root := t.TempDir()
	repoA := filepath.Join(root, "a")
	repoB := filepath.Join(root, "sub", "b")
	plain := filepath.Join(root, "plain")
	mkRepo(t, repoA)
	mkRepo(t, repoB)
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}

	// A worktree-style repo where .git is a *file*.
	repoC := filepath.Join(root, "c")
	if err := os.MkdirAll(repoC, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoC, ".git"), []byte("gitdir: /elsewhere/.git\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	repos, err := Find([]string{root}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(repos, repoA) || !contains(repos, repoB) || !contains(repos, repoC) {
		t.Errorf("missing expected repos in %v", repos)
	}
	if contains(repos, plain) {
		t.Errorf("plain dir wrongly reported as repo")
	}
	if len(repos) != 3 {
		t.Errorf("found %d repos, want 3: %v", len(repos), repos)
	}
}

func TestFindNested(t *testing.T) {
	root := t.TempDir()
	outer := filepath.Join(root, "outer")
	inner := filepath.Join(outer, "vendor", "inner")
	mkRepo(t, outer)
	mkRepo(t, inner)

	// Default: do not descend into the outer repo.
	repos, err := Find([]string{root}, false)
	if err != nil {
		t.Fatal(err)
	}
	if contains(repos, inner) {
		t.Errorf("nested repo found without --nested")
	}
	if !contains(repos, outer) {
		t.Errorf("outer repo not found")
	}

	// With nested=true, both are found.
	repos, err = Find([]string{root}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(repos, inner) || !contains(repos, outer) {
		t.Errorf("nested discovery missed a repo: %v", repos)
	}
}

func TestFindRepoRootDirectly(t *testing.T) {
	root := t.TempDir()
	mkRepo(t, root)
	repos, err := Find([]string{root}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || !contains(repos, root) {
		t.Errorf("passing a repo path directly should find it: %v", repos)
	}
}
