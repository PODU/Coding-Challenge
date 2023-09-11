// Package status classifies differences between HEAD, the index, and the
// working tree.
package status

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gg/internal/ignore"
	idxpkg "gg/internal/index"
	"gg/internal/object"
	"gg/internal/repo"
)

type ChangeKind int

const (
	Added ChangeKind = iota
	Modified
	Deleted
)

func (k ChangeKind) String() string {
	switch k {
	case Added:
		return "new file"
	case Modified:
		return "modified"
	case Deleted:
		return "deleted"
	}
	return "unknown"
}

// Change is a single path classification.
type Change struct {
	Path string
	Kind ChangeKind
}

// Status is the full working-tree status.
type Status struct {
	Branch    string
	Unborn    bool // HEAD points at a branch with no commits
	Staged    []Change
	Unstaged  []Change
	Untracked []string
}

// Clean reports whether there is nothing to commit.
func (s *Status) Clean() bool {
	return len(s.Staged) == 0 && len(s.Unstaged) == 0 && len(s.Untracked) == 0
}

type meta struct {
	mode string
	oid  string
}

// Compute builds the status for a repository.
func Compute(r *repo.Repo) (*Status, error) {
	s := &Status{Branch: r.CurrentBranch()}

	headOID, err := r.HeadOID()
	if err != nil {
		return nil, err
	}
	s.Unborn = headOID == ""

	headMap := map[string]meta{}
	if headOID != "" {
		if err := flattenCommitTree(r.GitDir, headOID, headMap); err != nil {
			return nil, err
		}
	}

	idx, err := idxpkg.Read(r.IndexPath())
	if err != nil {
		return nil, err
	}
	indexMap := map[string]meta{}
	for _, e := range idx.Entries {
		indexMap[e.Path] = meta{mode: modeString(e.Mode), oid: e.OIDHex()}
	}

	// Staged: index vs HEAD.
	for path, im := range indexMap {
		if hm, ok := headMap[path]; !ok {
			s.Staged = append(s.Staged, Change{path, Added})
		} else if hm.oid != im.oid {
			s.Staged = append(s.Staged, Change{path, Modified})
		}
	}
	for path := range headMap {
		if _, ok := indexMap[path]; !ok {
			s.Staged = append(s.Staged, Change{path, Deleted})
		}
	}

	matcher, err := ignore.FromFile(filepath.Join(r.Worktree, ".gitignore"))
	if err != nil {
		return nil, err
	}
	worktreeFiles, err := scanWorktree(r.Worktree, matcher)
	if err != nil {
		return nil, err
	}
	wtSet := map[string]bool{}
	for _, p := range worktreeFiles {
		wtSet[p] = true
	}

	// Unstaged: worktree vs index.
	for _, e := range idx.Entries {
		full := filepath.Join(r.Worktree, filepath.FromSlash(e.Path))
		data, err := os.ReadFile(full)
		if err != nil {
			if os.IsNotExist(err) {
				s.Unstaged = append(s.Unstaged, Change{e.Path, Deleted})
				continue
			}
			return nil, err
		}
		if object.Hash(object.TypeBlob, data) != e.OIDHex() {
			s.Unstaged = append(s.Unstaged, Change{e.Path, Modified})
		}
	}

	// Untracked: worktree files not in the index.
	for _, p := range worktreeFiles {
		if _, ok := indexMap[p]; !ok {
			s.Untracked = append(s.Untracked, p)
		}
	}

	sortChanges(s.Staged)
	sortChanges(s.Unstaged)
	sort.Strings(s.Untracked)
	return s, nil
}

func sortChanges(c []Change) {
	sort.Slice(c, func(i, j int) bool { return c[i].Path < c[j].Path })
}

func modeString(m uint32) string {
	if m&0o111 != 0 {
		return object.ModeExec
	}
	return object.ModeFile
}

// flattenCommitTree resolves a commit to its tree and flattens it into pathMap.
func flattenCommitTree(gitDir, commitOID string, pathMap map[string]meta) error {
	t, payload, err := object.Read(gitDir, commitOID)
	if err != nil {
		return err
	}
	if t != object.TypeCommit {
		return nil
	}
	c, err := object.DecodeCommit(payload)
	if err != nil {
		return err
	}
	return flattenTree(gitDir, c.Tree, "", pathMap)
}

func flattenTree(gitDir, treeOID, prefix string, pathMap map[string]meta) error {
	t, payload, err := object.Read(gitDir, treeOID)
	if err != nil {
		return err
	}
	if t != object.TypeTree {
		return nil
	}
	entries, err := object.DecodeTree(payload)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name
		if prefix != "" {
			name = prefix + "/" + e.Name
		}
		if e.Mode == object.ModeTree {
			if err := flattenTree(gitDir, e.OID, name, pathMap); err != nil {
				return err
			}
		} else {
			pathMap[name] = meta{mode: e.Mode, oid: e.OID}
		}
	}
	return nil
}

// scanWorktree returns repo-relative POSIX paths of all non-ignored files.
func scanWorktree(root string, matcher *ignore.Matcher) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if info.IsDir() {
			if rel == ".git" || strings.HasPrefix(rel, ".git/") {
				return filepath.SkipDir
			}
			if matcher.Match(rel, true) {
				return filepath.SkipDir
			}
			return nil
		}
		if matcher.Match(rel, false) {
			return nil
		}
		files = append(files, rel)
		return nil
	})
	return files, err
}
