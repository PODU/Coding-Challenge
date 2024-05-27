// Package discovery finds git repositories under a set of roots. A repository
// is any directory with a .git entry; worktrees and submodules have a .git file
// instead of a directory.
package discovery

import (
	"io/fs"
	"os"
	"path/filepath"
)

// Repo is a discovered repository.
type Repo struct {
	Path string // absolute path of the working tree
	Name string // base name, for display
}

func hasGit(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// Find returns the repositories under roots. It doesn't look inside a
// repository it has found unless nested is set. Unreadable directories are
// skipped.
func Find(roots []string, nested bool) ([]Repo, error) {
	seen := make(map[string]struct{})
	var repos []Repo

	add := func(path string) {
		abs, err := filepath.Abs(path)
		if err != nil {
			abs = path
		}
		if _, ok := seen[abs]; ok {
			return
		}
		seen[abs] = struct{}{}
		repos = append(repos, Repo{Path: abs, Name: filepath.Base(abs)})
	}

	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			continue
		}

		walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				if d != nil && d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.IsDir() {
				return nil
			}
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			if hasGit(path) {
				add(path)
				if !nested {
					return filepath.SkipDir
				}
			}
			return nil
		})
		if walkErr != nil {
			return repos, walkErr
		}
	}
	return repos, nil
}
