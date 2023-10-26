package job

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAndAddRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "crontab")

	initial := "# a comment\n" +
		"\n" +
		"# name: backup\n" +
		"0 2 * * *\t/usr/local/bin/backup.sh --full\n" +
		"@daily echo hello\n"
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	jobs, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("got %d jobs, want 2", len(jobs))
	}
	if jobs[0].Expr != "0 2 * * *" || jobs[0].Command != "/usr/local/bin/backup.sh --full" {
		t.Errorf("job0 = %+v", jobs[0])
	}
	if jobs[0].Name != "backup" {
		t.Errorf("job0 name = %q, want backup", jobs[0].Name)
	}
	if jobs[1].Expr != "@daily" || jobs[1].Command != "echo hello" {
		t.Errorf("job1 = %+v", jobs[1])
	}

	// Add a valid job and ensure it round-trips.
	added, err := Add(path, "*/30 9-17 * * 1-5", "python report.py")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if added.Command != "python report.py" {
		t.Errorf("added command = %q", added.Command)
	}

	jobs2, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(jobs2) != 3 {
		t.Fatalf("after add got %d jobs, want 3", len(jobs2))
	}
	if jobs2[2].Command != "python report.py" {
		t.Errorf("reloaded added command = %q", jobs2[2].Command)
	}

	// Comments preserved.
	content, _ := os.ReadFile(path)
	if !strings.Contains(string(content), "# a comment") {
		t.Error("comment not preserved after add")
	}
}

func TestAddRejectsInvalidWithoutMutating(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "crontab")
	if err := os.WriteFile(path, []byte("0 0 * * *\techo ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)

	if _, err := Add(path, "99 * * * *", "echo bad"); err == nil {
		t.Fatal("expected error for invalid expression")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Errorf("file mutated by rejected add:\nbefore=%q\nafter=%q", before, after)
	}
}

func TestLoadMissingFile(t *testing.T) {
	jobs, err := Load(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("Load missing file: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("got %d jobs, want 0", len(jobs))
	}
}

func TestParseLineErrors(t *testing.T) {
	cases := []string{
		"* * *",               // too few fields, no command
		"0 0 * * *",           // valid expr but no command
		"99 0 * * * echo bad", // invalid minute
	}
	for _, c := range cases {
		if _, err := ParseLine(c); err == nil {
			t.Errorf("ParseLine(%q) = nil error, want error", c)
		}
	}
}
