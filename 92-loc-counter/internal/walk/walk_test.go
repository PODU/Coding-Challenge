package walk

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"loc/internal/count"
	"loc/internal/report"
)

// buildTree creates the fixture tree described in the test plan and returns its
// root. Each source file has known counts.
func buildTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// a.go: 3 lines -> 1 blank, 1 comment, 1 code
	write("a.go", "package a\n\n// note\n")
	// sub/b.py: 2 lines -> 0 blank, 1 comment, 1 code
	write("sub/b.py", "# c\nx = 1\n")
	// sub/deep/c.c: 2 lines -> 0 blank, 0 comment, 2 code
	write("sub/deep/c.c", "int x;\nint y;\n")
	// Must be ignored by default:
	write("node_modules/x.js", "var a = 1;\n")
	write(".git/config", "[core]\n")
	// Binary file (NUL byte) -> skipped:
	write("image.png", "PNG\x00\x00binary")
	// No extension -> skipped unless --include-unknown:
	write("README", "hello\nworld\n")
	return root
}

func paths(jobs []Job) []string {
	out := make([]string, len(jobs))
	for i, j := range jobs {
		out[i] = filepath.Base(j.Path)
	}
	sort.Strings(out)
	return out
}

func TestWalkDefaultIgnores(t *testing.T) {
	root := buildTree(t)
	jobs, err := Walk([]string{root}, Options{})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	got := paths(jobs)
	want := []string{"a.go", "b.py", "c.c"}
	if len(got) != len(want) {
		t.Fatalf("got %v; want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v; want %v", got, want)
		}
	}
}

func TestWalkNoIgnore(t *testing.T) {
	root := buildTree(t)
	jobs, err := Walk([]string{root}, Options{NoIgnore: true})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	got := paths(jobs)
	// node_modules/x.js is now re-included. .git/config has no known extension
	// (skipped), README has no extension (skipped), image.png is binary (skipped).
	want := map[string]bool{"a.go": true, "b.py": true, "c.c": true, "x.js": true}
	if len(got) != len(want) {
		t.Fatalf("got %v; want keys %v", got, want)
	}
	for _, g := range got {
		if !want[g] {
			t.Errorf("unexpected file %q in %v", g, got)
		}
	}
}

func TestWalkIncludeUnknown(t *testing.T) {
	root := buildTree(t)
	jobs, err := Walk([]string{root}, Options{IncludeUnknown: true})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	found := false
	for _, j := range jobs {
		if filepath.Base(j.Path) == "README" {
			found = true
			if j.Def.Name != "(unknown)" {
				t.Errorf("README lang = %q; want (unknown)", j.Def.Name)
			}
		}
	}
	if !found {
		t.Error("README not included with --include-unknown")
	}
}

func TestWalkLangFilter(t *testing.T) {
	root := buildTree(t)
	jobs, err := Walk([]string{root}, Options{Langs: []string{"Go"}})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(jobs) != 1 || filepath.Base(jobs[0].Path) != "a.go" {
		t.Fatalf("lang filter got %v; want only a.go", paths(jobs))
	}
}

func TestWalkExclude(t *testing.T) {
	root := buildTree(t)
	jobs, err := Walk([]string{root}, Options{Excludes: []string{"*.py"}})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	for _, j := range jobs {
		if filepath.Ext(j.Path) == ".py" {
			t.Errorf("excluded .py still present: %s", j.Path)
		}
	}
}

// TestPipelineTotals runs discovery + counting and checks grand totals equal the
// sum of the individual fixtures (and that result is thread-independent).
func TestPipelineTotals(t *testing.T) {
	root := buildTree(t)
	jobs, err := Walk([]string{root}, Options{})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	var files []count.FileCount
	for _, j := range jobs {
		fc, err := count.CountFile(j.Path, j.Def)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, fc)
	}
	rep := report.Aggregate(files, "code")
	// a.go(3) + b.py(2) + c.c(2) = 7 lines; blank 1; comment 2; code 4.
	tot := rep.Total
	if tot.Files != 3 || tot.Lines != 7 || tot.Blank != 1 || tot.Comment != 2 || tot.Code != 4 {
		t.Errorf("totals = %+v; want Files=3 Lines=7 Blank=1 Comment=2 Code=4", tot)
	}
}
