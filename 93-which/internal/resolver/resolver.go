// Package resolver finds an executable by name in a list of directories.
// The directories and the Windows PATHEXT extensions are passed in rather than
// read from the environment, so tests can use temp directories.
package resolver

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// SplitPath splits PATH into directories. Empty entries are dropped: on Unix
// they traditionally mean the current directory, which is not something
// `which` should search without being asked.
func SplitPath(pathEnv string) []string {
	raw := filepath.SplitList(pathEnv)
	dirs := make([]string, 0, len(raw))
	for _, d := range raw {
		if d == "" {
			continue
		}
		dirs = append(dirs, d)
	}
	return dirs
}

// DefaultExts is the Windows fallback when PATHEXT is unset. It is nil on
// other systems.
func DefaultExts() []string {
	if runtime.GOOS != "windows" {
		return nil
	}
	return []string{".COM", ".EXE", ".BAT", ".CMD", ".VBS", ".JS", ".WS", ".MSC"}
}

// SplitExts parses PATHEXT (".COM;.EXE;.BAT"). Outside Windows it always
// returns nil.
func SplitExts(pathext string) []string {
	if runtime.GOOS != "windows" {
		return nil
	}
	if strings.TrimSpace(pathext) == "" {
		return DefaultExts()
	}
	parts := strings.Split(pathext, ";")
	exts := make([]string, 0, len(parts))
	for _, e := range parts {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		exts = append(exts, e)
	}
	return exts
}

func hasExt(name string, exts []string) bool {
	lower := strings.ToLower(name)
	for _, e := range exts {
		if strings.HasSuffix(lower, strings.ToLower(e)) {
			return true
		}
	}
	return false
}

// candidateNames lists the file names to try. On Windows that is the name
// itself if it already has a known extension, then name+ext for each
// extension in PATHEXT order.
func candidateNames(name string, exts []string) []string {
	if len(exts) == 0 {
		return []string{name}
	}
	cands := make([]string, 0, len(exts)+1)
	if hasExt(name, exts) {
		cands = append(cands, name)
	}
	for _, e := range exts {
		cands = append(cands, name+e)
	}
	return cands
}

// IsExecutable reports whether path is a regular file that can be run. With
// no exts (Unix) that means any execute bit is set; on Windows, which has no
// execute bit, it means the name ends in one of exts.
func IsExecutable(path string, exts []string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	if len(exts) == 0 {
		return info.Mode().Perm()&0o111 != 0
	}
	return hasExt(path, exts)
}

// Resolve returns the first match for name, or every match if all is set,
// with duplicates removed. A name containing a path separator is checked as
// given and PATH isn't searched.
func Resolve(name string, dirs []string, exts []string, all bool) []string {
	if strings.ContainsRune(name, '/') ||
		strings.ContainsRune(name, os.PathSeparator) ||
		filepath.IsAbs(name) {
		if IsExecutable(name, exts) {
			return []string{name}
		}
		return nil
	}

	var matches []string
	seen := make(map[string]struct{})

	for _, dir := range dirs {
		for _, cand := range candidateNames(name, exts) {
			p := filepath.Join(dir, cand)
			if !IsExecutable(p, exts) {
				continue
			}
			key := dedupeKey(p)
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			matches = append(matches, p)
			if !all {
				return matches
			}
		}
	}
	return matches
}

// dedupeKey lowercases on Windows, where paths are case-insensitive.
func dedupeKey(p string) string {
	clean := filepath.Clean(p)
	if runtime.GOOS == "windows" {
		return strings.ToLower(clean)
	}
	return clean
}
