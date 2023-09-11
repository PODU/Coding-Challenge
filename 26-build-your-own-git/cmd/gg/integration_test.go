package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gg/internal/object"
	"gg/internal/repo"
)

// chdir switches into dir for the duration of the test.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
}

// capture runs fn with os.Stdout redirected and returns what it printed.
func capture(t *testing.T, fn func() int) (string, int) {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	code := fn()
	w.Close()
	os.Stdout = orig
	out := readAll(t, r)
	return out, code
}

func readAll(t *testing.T, r *os.File) string {
	t.Helper()
	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	return sb.String()
}

func mustRun(t *testing.T, args ...string) {
	t.Helper()
	if code := run(args); code != 0 {
		t.Fatalf("run %v exited %d", args, code)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestInitLayout(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	mustRun(t, "init")

	gitDir := filepath.Join(dir, ".git")
	for _, sub := range []string{"objects", "refs/heads", "refs/tags", "hooks", "HEAD", "config"} {
		if _, err := os.Stat(filepath.Join(gitDir, filepath.FromSlash(sub))); err != nil {
			t.Errorf("missing %s: %v", sub, err)
		}
	}
	head, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(head)) != "ref: refs/heads/"+repo.DefaultBranch {
		t.Fatalf("HEAD = %q", head)
	}
}

func TestLifecycle(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Setenv("GG_AUTHOR_NAME", "Test User")
	t.Setenv("GG_AUTHOR_EMAIL", "test@example.com")

	mustRun(t, "init")
	writeFile(t, filepath.Join(dir, "a.txt"), "hello\n")
	writeFile(t, filepath.Join(dir, ".gitignore"), "ignored.txt\n")
	writeFile(t, filepath.Join(dir, "ignored.txt"), "secret\n")

	// status: a.txt and .gitignore untracked, ignored.txt hidden.
	out, code := capture(t, func() int { return run([]string{"status", "--porcelain"}) })
	if code != 0 {
		t.Fatalf("status exited %d", code)
	}
	if !strings.Contains(out, "?? a.txt") {
		t.Fatalf("expected a.txt untracked:\n%s", out)
	}
	if strings.Contains(out, "ignored.txt") {
		t.Fatalf("ignored.txt should be hidden:\n%s", out)
	}

	// add and verify the blob is stored at the canonical loose-object path.
	mustRun(t, "add", "a.txt")
	helloOID := "ce013625030ba8dba906f756967f9e9ca394464a"
	objPath := filepath.Join(dir, ".git", "objects", helloOID[:2], helloOID[2:])
	if _, err := os.Stat(objPath); err != nil {
		t.Fatalf("blob object not written: %v", err)
	}

	out, _ = capture(t, func() int { return run([]string{"status", "--porcelain"}) })
	if !strings.Contains(out, "A  a.txt") {
		t.Fatalf("expected a.txt staged:\n%s", out)
	}

	// first commit (root).
	mustRun(t, "commit", "-m", "first commit")
	r, err := repo.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	firstOID, err := r.ReadRef("refs/heads/" + repo.DefaultBranch)
	if err != nil || firstOID == "" {
		t.Fatalf("no commit ref: %v / %q", err, firstOID)
	}

	// log shows the message.
	out, _ = capture(t, func() int { return run([]string{"log"}) })
	if !strings.Contains(out, "first commit") || !strings.Contains(out, firstOID) {
		t.Fatalf("log output unexpected:\n%s", out)
	}

	// modify and diff.
	writeFile(t, filepath.Join(dir, "a.txt"), "hello world\n")
	out, _ = capture(t, func() int { return run([]string{"diff"}) })
	if !strings.Contains(out, "-hello") || !strings.Contains(out, "+hello world") {
		t.Fatalf("diff output unexpected:\n%s", out)
	}

	// stage change and make a second commit that chains the parent.
	mustRun(t, "add", "a.txt")
	mustRun(t, "add", ".gitignore")
	mustRun(t, "commit", "-m", "second commit")
	secondOID, _ := r.ReadRef("refs/heads/" + repo.DefaultBranch)
	if secondOID == firstOID || secondOID == "" {
		t.Fatalf("second commit did not advance ref")
	}
	_, payload, err := object.Read(r.GitDir, secondOID)
	if err != nil {
		t.Fatal(err)
	}
	c, err := object.DecodeCommit(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Parents) != 1 || c.Parents[0] != firstOID {
		t.Fatalf("second commit parent = %v, want [%s]", c.Parents, firstOID)
	}

	// working tree clean now.
	out, _ = capture(t, func() int { return run([]string{"status", "--porcelain"}) })
	if strings.TrimSpace(out) != "" {
		t.Fatalf("expected clean tree, got:\n%s", out)
	}
}

func TestOutsideRepoExitsUsage(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if code := run([]string{"status"}); code != 2 {
		t.Fatalf("status outside repo should exit 2, got %d", code)
	}
}
