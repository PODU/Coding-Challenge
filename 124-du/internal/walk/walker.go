package walk

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"du/internal/platform"
)

// Walker sums sizes bottom-up. Use one Walker for all the command-line paths
// so hardlinks and -L cycles are tracked across all of them.
type Walker struct {
	opts    Options
	sizer   *Sizer
	excl    *Excluder
	errOut  io.Writer
	visited map[string]bool // real paths of directories seen under -L

	hadError bool
	rootDev  int64
}

// New returns a Walker that reports unreadable paths to errOut.
func New(opts Options, errOut io.Writer) *Walker {
	return &Walker{
		opts:    opts,
		sizer:   NewSizer(opts.Apparent, opts.Inodes),
		excl:    NewExcluder(opts.Excludes),
		errOut:  errOut,
		visited: make(map[string]bool),
	}
}

// HadError reports whether any path could not be read.
func (w *Walker) HadError() bool { return w.hadError }

func (w *Walker) errf(path string, err error) {
	w.hadError = true
	fmt.Fprintf(w.errOut, "du: cannot access '%s': %v\n", path, err)
}

// Walk returns the entries under root, deepest first, and root's total.
func (w *Walker) Walk(root string) ([]Entry, int64) {
	root = filepath.Clean(root)
	if w.opts.OneFileSystem {
		if li, err := os.Lstat(root); err == nil {
			w.rootDev = platform.Stat(li).Dev
		}
	}
	var buf []Entry
	total := w.walk(root, 0, &buf)
	return buf, total
}

func (w *Walker) walk(path string, depth int, buf *[]Entry) int64 {
	li, err := os.Lstat(path)
	if err != nil {
		w.errf(path, err)
		return 0
	}
	if w.excl.Match(li.Name(), path) {
		return 0
	}

	// Without -L a symlink counts as a small file and is never followed.
	info := li
	if li.Mode()&os.ModeSymlink != 0 && w.opts.Dereference {
		ti, err := os.Stat(path)
		if err != nil {
			w.errf(path, err)
			return 0
		}
		info = ti
	}

	if !info.IsDir() {
		sz := w.sizer.Size(info)
		if w.opts.All || depth == 0 {
			*buf = append(*buf, Entry{Path: path, Size: sz, Depth: depth, IsDir: false})
		}
		return sz
	}

	// Under -L a symlink can point back up the tree, so skip directories we
	// have already been in.
	if w.opts.Dereference {
		if real, e := filepath.EvalSymlinks(path); e == nil && real != "" {
			if w.visited[real] {
				return 0
			}
			w.visited[real] = true
		}
	}

	total := w.sizer.Size(info)
	entries, err := os.ReadDir(path)
	if err != nil {
		// Report it, but still print the directory with whatever we got.
		w.errf(path, err)
	}
	for _, e := range entries {
		child := filepath.Join(path, e.Name())
		if w.opts.OneFileSystem {
			if ci, err := os.Lstat(child); err == nil && platform.Stat(ci).Dev != w.rootDev {
				continue
			}
		}
		total += w.walk(child, depth+1, buf)
	}
	*buf = append(*buf, Entry{Path: path, Size: total, Depth: depth, IsDir: true})
	return total
}
