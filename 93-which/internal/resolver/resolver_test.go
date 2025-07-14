package resolver

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// writeFile creates a regular file at dir/name with the given mode and returns
// its full path.
func writeFile(t *testing.T, dir, name string, mode os.FileMode) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), mode); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	// WriteFile honors umask; force the mode for executables on Unix.
	if err := os.Chmod(p, mode); err != nil {
		t.Fatalf("chmod %s: %v", p, err)
	}
	return p
}

// makeDir creates a subdirectory dir/name and returns its full path.
func makeDir(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", p, err)
	}
	return p
}

// TestResolveWithExtensions exercises the resolver using an injected extension
// set (the Windows PATHEXT rule). Because executability here is decided purely
// by filename extension + regular-file-ness, these cases run identically on
// every OS, giving us deterministic coverage of ordering, -a, de-duplication,
// directory rejection, and the PATHEXT extension preference.
func TestResolveWithExtensions(t *testing.T) {
	root := t.TempDir()
	d1 := makeDir(t, root, "d1")
	d2 := makeDir(t, root, "d2")
	d3 := makeDir(t, root, "d3")

	// Lowercase here to match the on-disk fixture filenames exactly; the
	// resolver returns the candidate it constructs (dir + name + ext), so
	// keeping the case aligned makes assertions exact. Case-insensitivity of
	// the match rule itself is covered by TestCandidateNamesExtAlreadyPresent
	// and TestSplitExts.
	exts := []string{".exe", ".bat"}

	// d1: tool.exe and tool.bat (to test extension preference within a dir).
	toolExe1 := writeFile(t, d1, "tool.exe", 0o644)
	writeFile(t, d1, "tool.bat", 0o644)
	// d2: tool.exe (to test first-match vs -a across dirs).
	toolExe2 := writeFile(t, d2, "tool.exe", 0o644)
	// d2: a plain file with an unrecognized extension — never executable.
	writeFile(t, d2, "plainfile.txt", 0o644)
	// d3: a *directory* named like a candidate — must be rejected.
	makeDir(t, d3, "tool.exe")

	tests := []struct {
		name string
		prog string
		dirs []string
		all  bool
		want []string
	}{
		{
			name: "first match honors PATH and PATHEXT order",
			prog: "tool",
			dirs: []string{d1, d2},
			all:  false,
			want: []string{toolExe1}, // .EXE before .BAT, d1 before d2
		},
		{
			name: "all matches across dirs in order",
			prog: "tool",
			dirs: []string{d1, d2},
			all:  true,
			// d1/tool.exe, d1/tool.bat, then d2/tool.exe
			want: []string{toolExe1, filepath.Join(d1, "tool.bat"), toolExe2},
		},
		{
			name: "not found",
			prog: "ghost",
			dirs: []string{d1, d2, d3},
			all:  true,
			want: nil,
		},
		{
			name: "non-executable extension ignored",
			prog: "plainfile",
			dirs: []string{d2},
			all:  true,
			want: nil,
		},
		{
			name: "directory shadowing the name is skipped",
			prog: "tool",
			dirs: []string{d3},
			all:  true,
			want: nil,
		},
		{
			name: "duplicate PATH dirs de-duplicate hits",
			prog: "tool",
			dirs: []string{d2, d2},
			all:  true,
			want: []string{toolExe2},
		},
		{
			name: "empty PATH finds nothing for a plain name",
			prog: "tool",
			dirs: nil,
			all:  true,
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Resolve(tc.prog, tc.dirs, exts, tc.all)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Resolve(%q) = %v, want %v", tc.prog, got, tc.want)
			}
		})
	}
}

// TestResolveAbsolutePath verifies that an argument containing a path
// separator bypasses PATH and is validated directly.
func TestResolveAbsolutePath(t *testing.T) {
	root := t.TempDir()
	exts := []string{".EXE"}

	good := writeFile(t, root, "real.exe", 0o644)
	makeDir(t, root, "adir.exe") // a directory, not a file

	t.Run("valid absolute path returns itself", func(t *testing.T) {
		got := Resolve(good, []string{root}, exts, false)
		want := []string{good}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Resolve(%q) = %v, want %v", good, got, want)
		}
	})

	t.Run("absolute path to a directory is rejected", func(t *testing.T) {
		dir := filepath.Join(root, "adir.exe")
		if got := Resolve(dir, []string{root}, exts, false); got != nil {
			t.Errorf("Resolve(%q) = %v, want nil", dir, got)
		}
	})

	t.Run("absolute path that does not exist is rejected", func(t *testing.T) {
		missing := filepath.Join(root, "nope.exe")
		if got := Resolve(missing, nil, exts, false); got != nil {
			t.Errorf("Resolve(%q) = %v, want nil", missing, got)
		}
	})
}

func TestSplitPath(t *testing.T) {
	sep := string(os.PathListSeparator)
	in := "a" + sep + sep + "b" + sep + "c"
	got := SplitPath(in)
	want := []string{"a", "b", "c"} // empty entry skipped
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SplitPath(%q) = %v, want %v", in, got, want)
	}
	if got := SplitPath(""); len(got) != 0 {
		t.Errorf("SplitPath(\"\") = %v, want empty", got)
	}
}

func TestSplitExts(t *testing.T) {
	if runtime.GOOS != "windows" {
		if got := SplitExts(".EXE;.BAT"); got != nil {
			t.Errorf("SplitExts on non-windows = %v, want nil", got)
		}
		return
	}
	got := SplitExts(".EXE;;.BAT;")
	want := []string{".EXE", ".BAT"} // empties skipped
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SplitExts = %v, want %v", got, want)
	}
	if got := SplitExts(""); !reflect.DeepEqual(got, DefaultExts()) {
		t.Errorf("SplitExts(\"\") = %v, want default %v", got, DefaultExts())
	}
}

// TestResolveCaseInsensitiveExt verifies that an uppercase PATHEXT entry
// matches a lowercase on-disk filename (Windows extension matching is
// case-insensitive). The returned path carries the constructed extension case.
func TestResolveCaseInsensitiveExt(t *testing.T) {
	if runtime.GOOS != "windows" {
		// Relies on a case-insensitive filesystem to stat thing.EXE -> thing.exe.
		t.Skip("case-insensitive extension match is the Windows rule")
	}
	root := t.TempDir()
	d := makeDir(t, root, "bin")
	writeFile(t, d, "thing.exe", 0o644) // lowercase on disk

	got := Resolve("thing", []string{d}, []string{".EXE"}, false) // uppercase ext
	want := []string{filepath.Join(d, "thing.EXE")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Resolve = %v, want %v", got, want)
	}
}

// TestCandidateNamesExtAlreadyPresent ensures a name already carrying a
// recognized extension is probed verbatim first.
func TestCandidateNamesExtAlreadyPresent(t *testing.T) {
	got := candidateNames("tool.exe", []string{".EXE", ".BAT"})
	if len(got) == 0 || got[0] != "tool.exe" {
		t.Errorf("candidateNames first = %v, want tool.exe first", got)
	}
}
