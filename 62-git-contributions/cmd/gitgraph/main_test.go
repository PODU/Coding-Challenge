package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// captureStdout runs fn with os.Stdout redirected and returns what was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	fn()
	w.Close()
	os.Stdout = old
	return <-done
}

func buildRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(env []string, args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), env...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run(nil, "init", "-q")
	run(nil, "config", "user.name", "Smoke Tester")
	run(nil, "config", "user.email", "smoke@example.com")
	run(nil, "config", "commit.gpgsign", "false")

	commit := func(f, when string) {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(when), 0o644); err != nil {
			t.Fatal(err)
		}
		run(nil, "add", f)
		env := []string{"GIT_AUTHOR_DATE=" + when, "GIT_COMMITTER_DATE=" + when}
		run(env, "commit", "-q", "-m", "c "+f)
	}
	commit("a.txt", "2026-03-01T10:00:00Z")
	commit("b.txt", "2026-03-01T15:00:00Z")
	commit("c.txt", "2026-03-03T10:00:00Z")
	return dir
}

func TestEndToEndSmoke(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not on PATH")
	}
	dir := buildRepo(t)

	args := []string{
		"--all-authors", "--no-cache", "--no-color", "--tz", "UTC",
		"--since", "2026-03-01", "--until", "2026-03-10", dir,
	}
	out := captureStdout(t, func() {
		if code := run(args); code != 0 {
			t.Errorf("run exit code = %d, want 0", code)
		}
	})

	// 3 commits over 2 active days (Mar 1 x2, Mar 3 x1).
	if !strings.Contains(out, "Total:    3 commits in 10 days") {
		t.Errorf("stats line missing/wrong:\n%s", out)
	}
	if !strings.Contains(out, "Active:   2 days") {
		t.Errorf("active days wrong:\n%s", out)
	}
	if !strings.Contains(out, "Busiest:  2026-03-01 (2)") {
		t.Errorf("busiest wrong:\n%s", out)
	}
}

func TestSVGFormatToFile(t *testing.T) {
	if !gitAvailable() {
		t.Skip("git not on PATH")
	}
	dir := buildRepo(t)
	outFile := filepath.Join(t.TempDir(), "g.svg")
	args := []string{
		"--all-authors", "--no-cache", "--tz", "UTC",
		"--since", "2026-03-01", "--until", "2026-03-10",
		"--format", "svg", "--out", outFile, dir,
	}
	if code := run(args); code != 0 {
		t.Fatalf("run exit code = %d", code)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "<svg") {
		t.Errorf("SVG output not written: %.40s", data)
	}
}

func TestUsageErrors(t *testing.T) {
	if code := run([]string{"--days", "notanumber"}); code != 2 {
		t.Errorf("bad --days exit = %d, want 2", code)
	}
	if code := run([]string{"--bogus-flag"}); code != 2 {
		t.Errorf("unknown flag exit = %d, want 2", code)
	}
}

func TestHelpAndVersion(t *testing.T) {
	out := captureStdout(t, func() {
		if code := run([]string{"--help"}); code != 0 {
			t.Errorf("--help exit = %d", code)
		}
	})
	if !strings.Contains(out, "Usage:") {
		t.Errorf("help text missing")
	}
	out = captureStdout(t, func() {
		if code := run([]string{"--version"}); code != 0 {
			t.Errorf("--version exit = %d", code)
		}
	})
	if !strings.Contains(out, "gitgraph") {
		t.Errorf("version text missing")
	}
}
