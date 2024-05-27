package gitlog

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestParseLog(t *testing.T) {
	data := []byte(
		"h1\x1fAda Lovelace\x1fada@x.io\x1f2026-03-01T10:00:00-05:00\x1e\n" +
			"h2\x1fBob\x1fbob@y.io\x1f2026-03-02T12:30:00Z\x1e\n")
	commits, err := ParseLog(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 {
		t.Fatalf("got %d commits, want 2", len(commits))
	}
	if commits[0].SHA != "h1" || commits[0].AuthorName != "Ada Lovelace" || commits[0].AuthorEmail != "ada@x.io" {
		t.Errorf("commit[0] = %+v", commits[0])
	}
	want := time.Date(2026, 3, 1, 10, 0, 0, 0, time.FixedZone("", -5*3600))
	if !commits[0].When.Equal(want) {
		t.Errorf("commit[0].When = %v, want %v", commits[0].When, want)
	}
}

func TestParseReflogLine(t *testing.T) {
	line := "0000000000000000000000000000000000000000 abc123 Ada <ada@x.io> 1740834000 -0500\tcommit (initial): hello"
	c, ok := parseReflogLine(line)
	if !ok {
		t.Fatal("failed to parse reflog line")
	}
	if c.SHA != "abc123" || c.AuthorName != "Ada" || c.AuthorEmail != "ada@x.io" {
		t.Errorf("parsed = %+v", c)
	}
	if c.When.Unix() != 1740834000 {
		t.Errorf("When unix = %d, want 1740834000", c.When.Unix())
	}
}

// makeRepo builds a real temporary git repository with pinned commit dates.
func makeRepo(t *testing.T) string {
	t.Helper()
	if !GitAvailable() {
		t.Skip("git not on PATH; skipping integration test")
	}
	dir := t.TempDir()

	run := func(env []string, args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run(nil, "init", "-q")
	run(nil, "config", "user.name", "Test Author")
	run(nil, "config", "user.email", "test@example.com")
	run(nil, "config", "commit.gpgsign", "false")

	commitAt := func(file, when string) {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(when), 0o644); err != nil {
			t.Fatal(err)
		}
		run(nil, "add", file)
		env := []string{"GIT_AUTHOR_DATE=" + when, "GIT_COMMITTER_DATE=" + when}
		run(env, "commit", "-q", "-m", "commit "+file)
	}

	commitAt("a.txt", "2026-03-01T10:00:00-05:00")
	commitAt("b.txt", "2026-03-01T20:00:00-05:00")
	commitAt("c.txt", "2026-03-03T09:00:00-05:00")

	return dir
}

func TestExecReaderIntegration(t *testing.T) {
	dir := makeRepo(t)

	commits, err := ExecReader{}.Read(dir, time.Time{}, ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 3 {
		t.Fatalf("got %d commits, want 3", len(commits))
	}
	for _, c := range commits {
		if c.SHA == "" {
			t.Error("empty SHA")
		}
		if c.AuthorEmail != "test@example.com" || c.AuthorName != "Test Author" {
			t.Errorf("unexpected author: %+v", c)
		}
	}
	// Commits are returned newest-first; check the oldest pinned date round-trips.
	oldest := commits[len(commits)-1]
	want := time.Date(2026, 3, 1, 10, 0, 0, 0, time.FixedZone("", -5*3600))
	if !oldest.When.Equal(want) {
		t.Errorf("oldest.When = %v, want %v", oldest.When, want)
	}

	if e := UserEmail(dir); e != "test@example.com" {
		t.Errorf("UserEmail = %q, want test@example.com", e)
	}
}

func TestExecReaderRange(t *testing.T) {
	dir := makeRepo(t)
	all, err := ExecReader{}.Read(dir, time.Time{}, ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// Commits after the oldest should be 2.
	oldestSHA := all[len(all)-1].SHA
	rng, err := ExecReader{}.ReadRange(dir, oldestSHA, ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rng) != 2 {
		t.Errorf("range after oldest = %d commits, want 2", len(rng))
	}
}

func TestEmptyRepoIsNotError(t *testing.T) {
	if !GitAvailable() {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	cmd := exec.Command("git", "-C", dir, "init", "-q")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	commits, err := ExecReader{}.Read(dir, time.Time{}, ReadOptions{})
	if err != nil {
		t.Errorf("empty repo should not error, got %v", err)
	}
	if len(commits) != 0 {
		t.Errorf("empty repo should have 0 commits, got %d", len(commits))
	}
}
