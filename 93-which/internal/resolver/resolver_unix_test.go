//go:build !windows

package resolver

import (
	"path/filepath"
	"reflect"
	"testing"
)

// TestResolveUnixExecBit exercises the Unix executability rule (any execute
// permission bit set) using an empty extension set. It is build-tagged to
// non-Windows because Windows files have no meaningful execute permission bit.
func TestResolveUnixExecBit(t *testing.T) {
	root := t.TempDir()
	d1 := makeDir(t, root, "d1")
	d2 := makeDir(t, root, "d2")
	d3 := makeDir(t, root, "d3")

	// Executable in both d1 and d2 (mode 0755).
	exec1 := writeFile(t, d1, "tool", 0o755)
	exec2 := writeFile(t, d2, "tool", 0o755)
	// Non-executable regular file (mode 0644) — must be ignored.
	writeFile(t, d1, "plainfile", 0o644)
	// A directory named like the target — must be rejected.
	makeDir(t, d3, "tool")

	var noExts []string

	t.Run("first match honors PATH order", func(t *testing.T) {
		got := Resolve("tool", []string{d1, d2}, noExts, false)
		want := []string{exec1}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Resolve = %v, want %v", got, want)
		}
	})

	t.Run("all matches in order", func(t *testing.T) {
		got := Resolve("tool", []string{d1, d2}, noExts, true)
		want := []string{exec1, exec2}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Resolve = %v, want %v", got, want)
		}
	})

	t.Run("non-executable file ignored", func(t *testing.T) {
		if got := Resolve("plainfile", []string{d1}, noExts, true); got != nil {
			t.Errorf("Resolve = %v, want nil", got)
		}
	})

	t.Run("directory shadowing the name is skipped", func(t *testing.T) {
		if got := Resolve("tool", []string{d3}, noExts, true); got != nil {
			t.Errorf("Resolve = %v, want nil", got)
		}
	})

	t.Run("absolute exec path resolves; non-exec does not", func(t *testing.T) {
		if got := Resolve(exec1, nil, noExts, false); !reflect.DeepEqual(got, []string{exec1}) {
			t.Errorf("Resolve(abs exec) = %v, want %v", got, []string{exec1})
		}
		plain := filepath.Join(d1, "plainfile")
		if got := Resolve(plain, nil, noExts, false); got != nil {
			t.Errorf("Resolve(abs non-exec) = %v, want nil", got)
		}
	})

	// Sanity: IsExecutable directly reflects the exec bit.
	if !IsExecutable(exec1, noExts) {
		t.Errorf("IsExecutable(%q) = false, want true", exec1)
	}
	if IsExecutable(filepath.Join(d1, "plainfile"), noExts) {
		t.Errorf("IsExecutable(plainfile) = true, want false")
	}
}
