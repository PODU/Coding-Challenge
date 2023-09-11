//go:build gitcompat

// These tests cross-check gg against the canonical git binary. They are gated
// behind the "gitcompat" build tag so the default suite stays hermetic/offline:
//
//	go test -tags gitcompat ./cmd/gg/
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH; skipping compatibility test")
	}
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// TestGitFsckCleanOnGGRepo builds a repo entirely with gg and asserts canonical
// git can read it: fsck is clean and log/status agree.
func TestGitFsckCleanOnGGRepo(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	chdir(t, dir)
	t.Setenv("GG_AUTHOR_NAME", "Test")
	t.Setenv("GG_AUTHOR_EMAIL", "test@example.com")

	mustRun(t, "init")
	writeFile(t, filepath.Join(dir, "a.txt"), "hello\n")
	writeFile(t, filepath.Join(dir, "sub", "b.txt"), "nested\n")
	mustRun(t, "add", "a.txt")
	mustRun(t, "add", "sub")
	mustRun(t, "commit", "-m", "from gg")

	// git must consider the object database and index valid.
	fsck := git(t, dir, "fsck", "--strict")
	if strings.Contains(strings.ToLower(fsck), "error") {
		t.Fatalf("git fsck reported problems:\n%s", fsck)
	}

	// git log should see our commit message.
	logOut := git(t, dir, "log", "--oneline")
	if !strings.Contains(logOut, "from gg") {
		t.Fatalf("git log did not show gg commit:\n%s", logOut)
	}

	// git status should report a clean tree (gg wrote the index git trusts).
	statusOut := git(t, dir, "status", "--porcelain")
	if strings.TrimSpace(statusOut) != "" {
		t.Fatalf("git status not clean after gg commit:\n%s", statusOut)
	}
}

// TestGGReadsGitRepo is the reverse: git creates the repo, gg reads its status.
func TestGGReadsGitRepo(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	writeFile(t, filepath.Join(dir, "x.txt"), "content\n")
	git(t, dir, "add", "x.txt")
	git(t, dir, "commit", "-q", "-m", "git commit")

	chdir(t, dir)
	out, code := capture(t, func() int { return run([]string{"log"}) })
	if code != 0 {
		t.Fatalf("gg log on git repo exited %d", code)
	}
	if !strings.Contains(out, "git commit") {
		t.Fatalf("gg log did not read git-created commit:\n%s", out)
	}
}
