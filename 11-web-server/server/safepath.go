package server

import (
	"errors"
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

// ErrForbidden indicates the requested path escaped the document root and must
// be answered with 403 Forbidden.
var ErrForbidden = errors.New("forbidden")

// Resolve maps a request path to a file path under root, returning
// ErrForbidden for anything that would escape it. root must be absolute and
// cleaned (see New). Any query string is ignored.
func Resolve(root, urlPath string) (string, error) {
	if i := strings.IndexByte(urlPath, '?'); i >= 0 {
		urlPath = urlPath[:i]
	}

	decoded, err := url.PathUnescape(urlPath)
	if err != nil {
		return "", ErrForbidden
	}

	// Backslashes and NUL bytes are never valid in a URL path here and are a
	// common Windows-style traversal vector.
	if strings.ContainsRune(decoded, 0) || strings.ContainsRune(decoded, '\\') {
		return "", ErrForbidden
	}
	if !strings.HasPrefix(decoded, "/") {
		return "", ErrForbidden
	}

	// Clean the path in relative form: path.Clean on an absolute path silently
	// drops a leading "..", which would turn a traversal attempt into a 404
	// instead of a 403.
	rel := path.Clean(strings.TrimPrefix(decoded, "/"))
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return "", ErrForbidden
	}

	abs := filepath.Join(root, filepath.FromSlash(rel))
	if !contained(root, abs) {
		return "", ErrForbidden
	}
	return abs, nil
}

// contained reports whether abs is root itself or lives inside root.
func contained(root, abs string) bool {
	if abs == root {
		return true
	}
	prefix := root
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	return strings.HasPrefix(abs, prefix)
}
