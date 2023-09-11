// Package repo locates and creates the .git directory and reads/writes HEAD and
// refs.
package repo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultBranch is the branch created by init.
const DefaultBranch = "main"

// Repo is an opened repository.
type Repo struct {
	Worktree string // absolute path to the working tree root
	GitDir   string // absolute path to .git
}

// ErrNotARepo is returned when no repository is found.
var ErrNotARepo = errors.New("not a git repository (or any parent up to mount point)")

// Init creates a new repository under dir (created if needed). Re-initialising
// an existing repository is benign and preserves config/refs.
func Init(dir string) (*Repo, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, err
	}
	gitDir := filepath.Join(abs, ".git")

	dirs := []string{
		gitDir,
		filepath.Join(gitDir, "objects"),
		filepath.Join(gitDir, "objects", "info"),
		filepath.Join(gitDir, "objects", "pack"),
		filepath.Join(gitDir, "refs"),
		filepath.Join(gitDir, "refs", "heads"),
		filepath.Join(gitDir, "refs", "tags"),
		filepath.Join(gitDir, "hooks"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, err
		}
	}

	files := map[string]string{
		filepath.Join(gitDir, "HEAD"):        "ref: refs/heads/" + DefaultBranch + "\n",
		filepath.Join(gitDir, "description"): "Unnamed repository; edit this file 'description' to name the repository.\n",
		filepath.Join(gitDir, "config"): "[core]\n" +
			"\trepositoryformatversion = 0\n" +
			"\tfilemode = false\n" +
			"\tbare = false\n" +
			"\tlogallrefupdates = true\n",
	}
	for path, content := range files {
		// Do not clobber an existing HEAD/config on re-init.
		if _, err := os.Stat(path); err == nil {
			continue
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return nil, err
		}
	}

	return &Repo{Worktree: abs, GitDir: gitDir}, nil
}

// Open searches start and its ancestors for a .git directory.
func Open(start string) (*Repo, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return nil, err
	}
	for {
		gitDir := filepath.Join(abs, ".git")
		info, err := os.Stat(gitDir)
		if err == nil && info.IsDir() {
			return &Repo{Worktree: abs, GitDir: gitDir}, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return nil, ErrNotARepo
		}
		abs = parent
	}
}

// IndexPath returns the path to the index file.
func (r *Repo) IndexPath() string { return filepath.Join(r.GitDir, "index") }

// HEADRef returns the symbolic ref HEAD points at (e.g. "refs/heads/main").
// If HEAD is detached it returns an empty ref and the OID.
func (r *Repo) HEADRef() (ref string, oid string, err error) {
	data, err := os.ReadFile(filepath.Join(r.GitDir, "HEAD"))
	if err != nil {
		return "", "", err
	}
	s := strings.TrimSpace(string(data))
	if strings.HasPrefix(s, "ref: ") {
		return strings.TrimSpace(strings.TrimPrefix(s, "ref: ")), "", nil
	}
	return "", s, nil
}

// CurrentBranch returns the short branch name, or "" if detached.
func (r *Repo) CurrentBranch() string {
	ref, _, err := r.HEADRef()
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(ref, "refs/heads/")
}

// ReadRef resolves a ref to an OID. Returns ("", nil) for an unborn branch.
func (r *Repo) ReadRef(ref string) (string, error) {
	path := filepath.Join(r.GitDir, filepath.FromSlash(ref))
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// UpdateRef writes an OID to a ref (creating parent dirs).
func (r *Repo) UpdateRef(ref, oid string) error {
	path := filepath.Join(r.GitDir, filepath.FromSlash(ref))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(oid+"\n"), 0o644)
}

// HeadOID resolves HEAD to a commit OID, or "" for an unborn branch.
func (r *Repo) HeadOID() (string, error) {
	ref, oid, err := r.HEADRef()
	if err != nil {
		return "", err
	}
	if ref == "" {
		return oid, nil
	}
	return r.ReadRef(ref)
}

// RelPath converts an absolute or relative OS path into a repo-relative,
// forward-slash path suitable for the index and trees.
func (r *Repo) RelPath(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(r.Worktree, abs)
	if err != nil {
		return "", err
	}
	rel = filepath.ToSlash(rel)
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return "", fmt.Errorf("path %q is outside the repository", p)
	}
	return rel, nil
}
