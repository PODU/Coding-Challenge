// Package cmd implements create, extract and list. It is the only package that
// touches the filesystem.
package cmd

import (
	"fmt"
	"path/filepath"
	"strings"
)

// toArchiveName converts an OS path to archive form: forward slashes, no
// drive letter, no leading "/" or "./" (like tar, so archives extract
// relative to the current directory), and a trailing slash for directories.
func toArchiveName(p string, isDir bool) string {
	if vol := filepath.VolumeName(p); vol != "" {
		p = p[len(vol):]
	}
	p = filepath.ToSlash(p)
	p = strings.TrimLeft(p, "/")
	p = strings.TrimPrefix(p, "./")
	if isDir && !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}

// sanitizePath maps a member name to a path under dest, rejecting absolute
// paths, drive letters and anything that ".." would take outside dest.
func sanitizePath(dest, name string) (string, error) {
	clean := strings.TrimRight(filepath.ToSlash(name), "/")
	if clean == "" {
		return "", fmt.Errorf("empty member name")
	}

	if strings.HasPrefix(clean, "/") || filepath.IsAbs(name) {
		return "", fmt.Errorf("refusing absolute path %q", name)
	}
	if vol := filepath.VolumeName(filepath.FromSlash(clean)); vol != "" {
		return "", fmt.Errorf("refusing path with volume name %q", name)
	}

	target := filepath.Join(dest, filepath.FromSlash(clean))
	rel, err := filepath.Rel(dest, target)
	if err != nil {
		return "", fmt.Errorf("invalid path %q: %w", name, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("refusing path %q: escapes destination", name)
	}
	return target, nil
}

// linkEscapes reports whether a symlink at linkPath pointing to target would
// resolve outside dest. Absolute targets always count as escaping.
func linkEscapes(dest, linkPath, target string) bool {
	if filepath.IsAbs(target) || strings.HasPrefix(filepath.ToSlash(target), "/") {
		return true
	}
	if vol := filepath.VolumeName(filepath.FromSlash(target)); vol != "" {
		return true
	}
	resolved := filepath.Join(filepath.Dir(linkPath), filepath.FromSlash(target))
	rel, err := filepath.Rel(dest, resolved)
	if err != nil {
		return true
	}
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
