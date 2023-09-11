package ignore

import "testing"

func TestMatch(t *testing.T) {
	m := New([]string{
		"*.log",
		"build/",
		"node_modules",
		"/anchored.txt",
		"!keep.log",
		"# a comment",
		"",
		"docs/*.tmp",
	})

	cases := []struct {
		path  string
		isDir bool
		want  bool
	}{
		{"app.log", false, true},
		{"sub/app.log", false, true},
		{"keep.log", false, false},   // negated
		{"build", true, true},        // dir pattern matches dir
		{"build/out.o", false, true}, // inside ignored dir component
		{"node_modules", true, true},
		{"node_modules/x/y.js", false, true},
		{"anchored.txt", false, true},
		{"sub/anchored.txt", false, false}, // anchored to root only
		{"docs/a.tmp", false, true},
		{"docs/sub/a.tmp", false, false}, // single-level glob
		{"main.go", false, false},
		{".git", true, true},
		{".git/config", false, true},
	}
	for _, c := range cases {
		got := m.Match(c.path, c.isDir)
		if got != c.want {
			t.Errorf("Match(%q, dir=%v) = %v, want %v", c.path, c.isDir, got, c.want)
		}
	}
}

func TestAlwaysIgnoreGit(t *testing.T) {
	m := New(nil)
	if !m.Match(".git/objects/ab/cd", false) {
		t.Fatal(".git contents must always be ignored")
	}
}
