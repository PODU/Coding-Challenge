// Package gitlog reads commit history, normally by running git. If git isn't
// installed it falls back to reading the HEAD reflog directly.
package gitlog

import (
	"os/exec"
	"time"

	"gitgraph/internal/model"
)

type ReadOptions struct {
	IncludeMerges bool
	AllBranches   bool // git log --all
}

// Reader reads commits authored on or after since from a repository.
type Reader interface {
	Read(repoPath string, since time.Time, opts ReadOptions) ([]model.Commit, error)
}

// GitAvailable reports whether git is on PATH.
func GitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// Default returns an ExecReader if git is installed, else a DirectReader.
func Default() Reader {
	if GitAvailable() {
		return ExecReader{}
	}
	return DirectReader{}
}

// UserEmail returns the repo's user.email setting, or "" if there is none.
func UserEmail(repoPath string) string {
	if !GitAvailable() {
		return ""
	}
	out, err := exec.Command("git", "-C", repoPath, "config", "user.email").Output()
	if err != nil {
		return ""
	}
	return string(trimNL(out))
}

func trimNL(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}
