// Package index scans a folder into path -> (size, mtime, hash) and saves it.
// The saved index is also the last-synced baseline.
package index

import (
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"dropbox/internal/atomic"
	"dropbox/internal/hash"
)

// FileMeta is what we remember about one file.
type FileMeta struct {
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mtime"`
	Hash    string    `json:"hash"`
}

// Index maps a slash-separated relative path to its metadata.
type Index map[string]FileMeta

// Paths returns the index's paths, sorted.
func (idx Index) Paths() []string {
	out := make([]string, 0, len(idx))
	for p := range idx {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// Load reads an index from disk. A missing file gives an empty index.
func Load(path string) (Index, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Index{}, nil
		}
		return nil, err
	}
	idx := Index{}
	if len(b) == 0 {
		return idx, nil
	}
	if err := json.Unmarshal(b, &idx); err != nil {
		return nil, err
	}
	return idx, nil
}

func (idx Index) Save(path string) error {
	b, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return atomic.WriteBytes(path, 0o644, b)
}

// HashFunc hashes the file at an absolute path; tests replace it.
type HashFunc func(path string) (string, error)

// DefaultIgnore are patterns ignored on every scan.
var DefaultIgnore = []string{".dropbox/"}

// Scan walks root and builds an Index, re-hashing a file only if its size or
// mtime differs from prev. Symlinks and other non-regular files are skipped.
func Scan(root string, ignore []string, prev Index, hasher HashFunc) (Index, error) {
	if hasher == nil {
		hasher = hash.File
	}
	patterns := append(append([]string{}, DefaultIgnore...), ignore...)
	out := Index{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		rel, rerr := filepath.Rel(root, p)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if Ignored(rel+"/", patterns) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil // do not follow symlinks
		}
		if Ignored(rel, patterns) {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		meta := FileMeta{Size: info.Size(), ModTime: info.ModTime()}
		if pm, ok := prev[rel]; ok && pm.Size == meta.Size && pm.ModTime.Equal(meta.ModTime) && pm.Hash != "" {
			meta.Hash = pm.Hash
		} else {
			h, herr := hasher(p)
			if herr != nil {
				return herr
			}
			meta.Hash = h
		}
		out[rel] = meta
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Ignored reports whether the slash path rel matches any ignore pattern.
// A pattern ending in "/" matches a directory (by exact path, prefix, or any
// path component of that name). Other patterns are glob-matched against the
// base name and the full relative path.
func Ignored(rel string, patterns []string) bool {
	r := strings.TrimSuffix(rel, "/")
	base := path.Base(r)
	for _, pat := range patterns {
		if pat == "" {
			continue
		}
		if strings.HasSuffix(pat, "/") {
			dir := strings.TrimSuffix(pat, "/")
			if r == dir || strings.HasPrefix(r, dir+"/") || base == dir {
				return true
			}
			continue
		}
		if ok, _ := path.Match(pat, base); ok {
			return true
		}
		if ok, _ := path.Match(pat, r); ok {
			return true
		}
	}
	return false
}
