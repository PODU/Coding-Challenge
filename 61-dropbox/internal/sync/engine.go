// Package sync decides what to transfer by comparing the local index, the
// remote index and the last-synced baseline. It does no I/O.
package sync

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"dropbox/internal/index"
	"dropbox/internal/wire"
)

// Entry is the minimal per-path information the planner needs.
type Entry struct {
	Hash    string
	Size    int64
	Version int64 // only meaningful for the remote snapshot
}

// Snapshot maps a slash path to an Entry.
type Snapshot map[string]Entry

// FromIndex converts a local/baseline index into a Snapshot.
func FromIndex(idx index.Index) Snapshot {
	s := make(Snapshot, len(idx))
	for p, m := range idx {
		s[p] = Entry{Hash: m.Hash, Size: m.Size}
	}
	return s
}

// FromRemote converts the server's file list into a Snapshot.
func FromRemote(files []wire.FileInfo) Snapshot {
	s := make(Snapshot, len(files))
	for _, f := range files {
		s[f.Path] = Entry{Hash: f.Hash, Size: f.Size, Version: f.Version}
	}
	return s
}

type Kind int

const (
	Upload Kind = iota
	Download
	DeleteLocal
	DeleteRemote
	Conflict
)

func (k Kind) String() string {
	switch k {
	case Upload:
		return "upload"
	case Download:
		return "download"
	case DeleteLocal:
		return "delete-local"
	case DeleteRemote:
		return "delete-remote"
	case Conflict:
		return "conflict"
	}
	return "unknown"
}

type Action struct {
	Kind          Kind
	Path          string
	RemoteVersion int64  // current remote version, used as the If-Match precondition
	ConflictPath  string // sidecar path, set only for Conflict
}

// Plan returns the actions needed to bring both sides in line, sorted by path
// and kind. Each side is compared against the baseline:
//
//	local      remote        -> action
//	unchanged  unchanged      -> no-op
//	changed    unchanged      -> upload
//	unchanged  changed        -> download
//	new        absent         -> upload
//	absent     new            -> download
//	deleted    unchanged      -> delete remote
//	unchanged  deleted        -> delete local
//	changed    changed (==)   -> no-op (converged)
//	changed    changed (!=)   -> conflict
//	changed    deleted        -> upload (an edit beats a delete)
//	deleted    changed        -> download (likewise)
func Plan(local, remote, base Snapshot, now time.Time) []Action {
	paths := make(map[string]struct{}, len(local)+len(remote)+len(base))
	for p := range local {
		paths[p] = struct{}{}
	}
	for p := range remote {
		paths[p] = struct{}{}
	}
	for p := range base {
		paths[p] = struct{}{}
	}

	var actions []Action
	for p := range paths {
		l, lok := local[p]
		r, rok := remote[p]
		b, bok := base[p]

		switch {
		case lok && rok:
			if l.Hash == r.Hash {
				continue // both sides already agree
			}
			lChanged := !bok || l.Hash != b.Hash
			rChanged := !bok || r.Hash != b.Hash
			switch {
			case lChanged && rChanged:
				actions = append(actions, Action{
					Kind: Conflict, Path: p, RemoteVersion: r.Version,
					ConflictPath: conflictName(p, now),
				})
			case lChanged:
				actions = append(actions, Action{Kind: Upload, Path: p, RemoteVersion: r.Version})
			case rChanged:
				actions = append(actions, Action{Kind: Download, Path: p, RemoteVersion: r.Version})
			}

		case lok && !rok:
			switch {
			case bok && l.Hash == b.Hash:
				actions = append(actions, Action{Kind: DeleteLocal, Path: p})
			default:
				// new local file, or local modified after a remote delete -> upload
				actions = append(actions, Action{Kind: Upload, Path: p, RemoteVersion: 0})
			}

		case !lok && rok:
			switch {
			case bok && r.Hash == b.Hash:
				actions = append(actions, Action{Kind: DeleteRemote, Path: p, RemoteVersion: r.Version})
			default:
				// new remote file, or remote modified after a local delete -> download
				actions = append(actions, Action{Kind: Download, Path: p, RemoteVersion: r.Version})
			}

		default:
			// Deleted on both sides.
		}
	}

	sort.Slice(actions, func(i, j int) bool {
		if actions[i].Path != actions[j].Path {
			return actions[i].Path < actions[j].Path
		}
		return actions[i].Kind < actions[j].Kind
	})
	return actions
}

// conflictName returns "name (conflicted copy YYYY-MM-DD HH-MM-SS).ext" in the
// same directory as path.
func conflictName(p string, now time.Time) string {
	dir := path.Dir(p)
	base := path.Base(p)
	ext := path.Ext(base)
	name := strings.TrimSuffix(base, ext)
	ts := now.Format("2006-01-02 15-04-05")
	nb := fmt.Sprintf("%s (conflicted copy %s)%s", name, ts, ext)
	if dir == "." || dir == "" {
		return nb
	}
	return dir + "/" + nb
}
