package client

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dropbox/internal/atomic"
	"dropbox/internal/hash"
	"dropbox/internal/index"
	"dropbox/internal/sync"
)

// Syncer carries out the planner's actions against the local folder and the
// server.
type Syncer struct {
	Root   string
	Ignore []string
	Client *Client
	Now    func() time.Time // used for conflict file names; tests fix it
	DryRun bool
	Logf   func(string, ...any)
}

type Result struct {
	Uploaded      int
	Downloaded    int
	DeletedLocal  int
	DeletedRemote int
	Conflicts     int
	Actions       []sync.Action
}

func (s *Syncer) logf(f string, a ...any) {
	if s.Logf != nil {
		s.Logf(f, a...)
	}
}

func (s *Syncer) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Syncer) indexPath() string {
	return filepath.Join(s.Root, ".dropbox", "index.json")
}

// Plan compares the local folder, the server and the last-synced baseline and
// returns what needs doing.
func (s *Syncer) Plan() ([]sync.Action, error) {
	base, err := index.Load(s.indexPath())
	if err != nil {
		return nil, err
	}
	local, err := index.Scan(s.Root, s.Ignore, base, hash.File)
	if err != nil {
		return nil, err
	}
	remote, err := s.Client.List()
	if err != nil {
		return nil, err
	}
	return sync.Plan(sync.FromIndex(local), sync.FromRemote(remote), sync.FromIndex(base), s.now()), nil
}

// SyncOnce runs one plan-and-transfer pass. Only paths that end up identical
// on both sides go into the new baseline, so anything that failed is retried
// next time.
func (s *Syncer) SyncOnce() (Result, error) {
	plan, err := s.Plan()
	if err != nil {
		return Result{}, err
	}
	var res Result
	res.Actions = plan
	if s.DryRun {
		for _, a := range plan {
			countAction(&res, a)
		}
		return res, nil
	}
	for _, a := range plan {
		if err := s.execute(a); err != nil {
			s.logf("action %s %s failed: %v", a.Kind, a.Path, err)
			continue
		}
		countAction(&res, a)
	}
	if err := s.saveBaseline(); err != nil {
		return res, err
	}
	return res, nil
}

// Watch runs SyncOnce now and then every interval until ctx is cancelled. It
// polls rather than using filesystem notifications.
func (s *Syncer) Watch(ctx context.Context, interval time.Duration) error {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		res, err := s.SyncOnce()
		if err != nil {
			s.logf("sync error: %v", err)
		} else if len(res.Actions) > 0 {
			s.logf("synced: %d up, %d down, %d del-local, %d del-remote, %d conflicts",
				res.Uploaded, res.Downloaded, res.DeletedLocal, res.DeletedRemote, res.Conflicts)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
}

func countAction(res *Result, a sync.Action) {
	switch a.Kind {
	case sync.Upload:
		res.Uploaded++
	case sync.Download:
		res.Downloaded++
	case sync.DeleteLocal:
		res.DeletedLocal++
	case sync.DeleteRemote:
		res.DeletedRemote++
	case sync.Conflict:
		res.Conflicts++
	}
}

func (s *Syncer) execute(a sync.Action) error {
	switch a.Kind {
	case sync.Upload:
		return s.upload(a.Path, a.RemoteVersion)
	case sync.Download:
		return s.download(a.Path)
	case sync.DeleteLocal:
		return s.deleteLocal(a.Path)
	case sync.DeleteRemote:
		return s.Client.Delete(a.Path, a.RemoteVersion)
	case sync.Conflict:
		return s.conflict(a)
	}
	return nil
}

// localPath joins rel onto the root and refuses any path that escapes it.
func (s *Syncer) localPath(rel string) (string, error) {
	clean := filepath.Clean(filepath.Join(s.Root, filepath.FromSlash(rel)))
	root := filepath.Clean(s.Root)
	if clean != root && !strings.HasPrefix(clean, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes sync root: %s", rel)
	}
	return clean, nil
}

func (s *Syncer) upload(rel string, version int64) error {
	lp, err := s.localPath(rel)
	if err != nil {
		return err
	}
	f, err := os.Open(lp)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	h, err := hash.File(lp)
	if err != nil {
		return err
	}
	_, err = s.Client.Upload(rel, version, h, st.Size(), f)
	return err
}

func (s *Syncer) download(rel string) error {
	lp, err := s.localPath(rel)
	if err != nil {
		return err
	}
	return atomic.WriteFile(lp, 0o644, func(w io.Writer) error {
		_, err := s.Client.Download(rel, w)
		return err
	})
}

func (s *Syncer) deleteLocal(rel string) error {
	lp, err := s.localPath(rel)
	if err != nil {
		return err
	}
	if err := os.Remove(lp); err != nil && !os.IsNotExist(err) {
		return err
	}
	s.pruneEmptyDirs(filepath.Dir(lp))
	return nil
}

func (s *Syncer) pruneEmptyDirs(dir string) {
	root := filepath.Clean(s.Root)
	for {
		d := filepath.Clean(dir)
		if d == root || !strings.HasPrefix(d, root+string(os.PathSeparator)) {
			return
		}
		entries, err := os.ReadDir(d)
		if err != nil || len(entries) > 0 {
			return
		}
		if err := os.Remove(d); err != nil {
			return
		}
		dir = filepath.Dir(d)
	}
}

// conflict saves the local edit as a "conflicted copy", takes the server's
// version at the original path, and uploads the copy so other clients get it.
func (s *Syncer) conflict(a sync.Action) error {
	lp, err := s.localPath(a.Path)
	if err != nil {
		return err
	}
	cp, err := s.localPath(a.ConflictPath)
	if err != nil {
		return err
	}
	if err := copyFile(lp, cp); err != nil {
		return err
	}
	if err := s.download(a.Path); err != nil {
		return err
	}
	return s.upload(a.ConflictPath, 0)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	return atomic.WriteFile(dst, 0o644, func(w io.Writer) error {
		_, err := io.Copy(w, in)
		return err
	})
}

// saveBaseline records only the paths that now match on both sides.
func (s *Syncer) saveBaseline() error {
	prev, _ := index.Load(s.indexPath())
	local, err := index.Scan(s.Root, s.Ignore, prev, hash.File)
	if err != nil {
		return err
	}
	remoteFiles, err := s.Client.List()
	if err != nil {
		return err
	}
	remote := sync.FromRemote(remoteFiles)
	baseline := index.Index{}
	for p, m := range local {
		if r, ok := remote[p]; ok && r.Hash == m.Hash {
			baseline[p] = m
		}
	}
	return baseline.Save(s.indexPath())
}
