package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

type walker struct {
	opts    Options
	seen    map[string]FileInfo // by absolute path, so overlapping roots count once
	visited map[string]bool     // resolved directories, to stop symlink loops
	errs    []ScanError
}

// walkRoots lists every regular file under the roots. Problems with individual
// paths are collected and returned rather than stopping the walk.
func walkRoots(roots []string, opts Options) ([]FileInfo, []ScanError) {
	w := &walker{
		opts:    opts,
		seen:    make(map[string]FileInfo),
		visited: make(map[string]bool),
	}
	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil {
			w.errs = append(w.errs, ScanError{root, err})
			continue
		}
		switch {
		case info.IsDir():
			if real, err := filepath.EvalSymlinks(root); err == nil {
				w.visited[real] = true
			}
			w.walk(root)
		case info.Mode().IsRegular():
			w.addFile(root, info)
		}
	}

	files := make([]FileInfo, 0, len(w.seen))
	for _, f := range w.seen {
		files = append(files, f)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, w.errs
}

func (w *walker) walk(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		w.errs = append(w.errs, ScanError{dir, err})
		return
	}
	for _, e := range entries {
		full := filepath.Join(dir, e.Name())
		info, err := e.Info()
		if err != nil {
			w.errs = append(w.errs, ScanError{full, err})
			continue
		}
		mode := info.Mode()
		switch {
		case mode&fs.ModeSymlink != 0:
			w.handleSymlink(full)
		case mode.IsDir():
			w.walk(full)
		case mode.IsRegular():
			w.addFile(full, info)
		}
	}
}

func (w *walker) handleSymlink(path string) {
	if !w.opts.FollowSymlinks {
		return
	}
	target, err := os.Stat(path) // follows the link
	if err != nil {
		w.errs = append(w.errs, ScanError{path, err})
		return
	}
	switch {
	case target.IsDir():
		real, err := filepath.EvalSymlinks(path)
		if err != nil {
			w.errs = append(w.errs, ScanError{path, err})
			return
		}
		if w.visited[real] {
			return // a loop, or already walked
		}
		w.visited[real] = true
		w.walk(path)
	case target.Mode().IsRegular():
		w.addFile(path, target)
	}
}

func (w *walker) addFile(path string, info os.FileInfo) {
	canon, err := filepath.Abs(path)
	if err != nil {
		canon = path
	}
	canon = filepath.Clean(canon)
	if _, ok := w.seen[canon]; ok {
		return
	}
	dev, ino := devIno(info)
	w.seen[canon] = FileInfo{Path: canon, Size: info.Size(), Dev: dev, Ino: ino}
}
