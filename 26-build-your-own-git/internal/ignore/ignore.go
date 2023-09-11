// Package ignore implements a minimal subset of .gitignore matching: literal
// names, '*' and '?' globs, anchored and directory patterns, and negation.
// The .git directory is always ignored.
package ignore

import (
	"bufio"
	"os"
	"path"
	"strings"
)

type pattern struct {
	text     string // glob, with no leading '/' or trailing '/'
	negate   bool
	dirOnly  bool
	anchored bool // pattern contained a non-trailing '/', match from root
}

// Matcher tests repo-relative POSIX paths against ignore rules.
type Matcher struct {
	patterns []pattern
}

// New builds a matcher from raw .gitignore lines.
func New(lines []string) *Matcher {
	m := &Matcher{}
	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r\n")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		p := pattern{text: line}
		if strings.HasPrefix(p.text, "!") {
			p.negate = true
			p.text = p.text[1:]
		}
		if strings.HasSuffix(p.text, "/") {
			p.dirOnly = true
			p.text = strings.TrimSuffix(p.text, "/")
		}
		// A '/' anywhere except the trailing position anchors the pattern.
		if strings.Contains(p.text, "/") {
			p.anchored = true
		}
		p.text = strings.TrimPrefix(p.text, "/")
		if p.text == "" {
			continue
		}
		m.patterns = append(m.patterns, p)
	}
	return m
}

// FromFile loads a .gitignore file; a missing file yields an empty matcher.
func FromFile(filePath string) (*Matcher, error) {
	f, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return New(nil), nil
		}
		return nil, err
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return New(lines), sc.Err()
}

// Match reports whether the given repo-relative path (forward slashes) is
// ignored. isDir indicates whether the path is a directory.
func (m *Matcher) Match(relPath string, isDir bool) bool {
	relPath = strings.TrimPrefix(relPath, "./")
	if relPath == ".git" || strings.HasPrefix(relPath, ".git/") {
		return true
	}

	ignored := false
	for _, p := range m.patterns {
		if m.matchPattern(p, relPath, isDir) {
			ignored = !p.negate
		}
	}
	return ignored
}

// matchPattern reports whether p matches the path or any of its ancestor
// directories (so an ignored directory excludes everything beneath it).
func (m *Matcher) matchPattern(p pattern, relPath string, isDir bool) bool {
	// The path itself (a dirOnly pattern only matches when it is a directory).
	if m.single(p, relPath) && (!p.dirOnly || isDir) {
		return true
	}
	// Any ancestor directory prefix (always a directory, so dirOnly is fine).
	parts := strings.Split(relPath, "/")
	for i := 1; i < len(parts); i++ {
		prefix := strings.Join(parts[:i], "/")
		if m.single(p, prefix) {
			return true
		}
	}
	return false
}

// single tests a pattern against one candidate path (no ancestor expansion).
func (m *Matcher) single(p pattern, candidate string) bool {
	if p.anchored {
		ok, _ := path.Match(p.text, candidate)
		return ok
	}
	if ok, _ := path.Match(p.text, path.Base(candidate)); ok {
		return true
	}
	parts := strings.Split(candidate, "/")
	for i := range parts {
		sub := strings.Join(parts[i:], "/")
		if ok, _ := path.Match(p.text, sub); ok {
			return true
		}
	}
	return false
}
