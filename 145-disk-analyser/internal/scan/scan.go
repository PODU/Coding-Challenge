package scan

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
)

// Progress is updated live while a scan runs and can be read from another
// goroutine.
type Progress struct {
	Items atomic.Int64
	Bytes atomic.Int64 // disk usage found so far (before hard-link dedupe)
	Dirs  atomic.Int64
}

type Warning struct {
	Path string
	Err  error
}

func (w Warning) String() string { return w.Path + ": " + ErrText(w.Err) }

// ErrText is an error without the operation and path that fs errors repeat.
func ErrText(err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	return err.Error()
}

type Options struct {
	Workers  int       // directories read at once; 0 means 4 per CPU
	OneFS    bool      // do not descend into other filesystems (Unix)
	Progress *Progress // optional
}

type scanner struct {
	opts     Options
	sem      chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
	warnings []Warning
	// Swappable for tests.
	readDir func(string) ([]os.DirEntry, error)
	lstat   func(string) (os.FileInfo, error)
}

func newScanner(o Options) *scanner {
	if o.Workers <= 0 {
		o.Workers = 4 * runtime.NumCPU()
	}
	if o.Progress == nil {
		o.Progress = &Progress{}
	}
	return &scanner{opts: o, sem: make(chan struct{}, o.Workers), readDir: os.ReadDir, lstat: os.Lstat}
}

// Scan reads the tree under root. Unreadable directories become warnings and
// the scan carries on. The root itself is followed if it is a symlink;
// nothing below it is.
func Scan(ctx context.Context, root string, o Options) (*Node, []Warning, error) {
	return newScanner(o).scan(ctx, root)
}

func (s *scanner) scan(ctx context.Context, root string) (*Node, []Warning, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, err
	}
	fi, err := os.Stat(abs)
	if err != nil {
		return nil, nil, err
	}
	if !fi.IsDir() {
		return nil, nil, errors.New(abs + " is not a directory")
	}
	n := newNode(abs, nil, abs, fi)
	n.Kind = Dir
	s.dir(ctx, n, abs, n.ID.Dev)
	s.wg.Wait()
	Recompute(n)
	return n, s.warnings, ctx.Err()
}

// Rescan rereads the directory n in place, replacing its children. Call
// Recompute on the root afterwards.
func Rescan(ctx context.Context, n *Node, o Options) ([]Warning, error) {
	if n.Kind != Dir {
		return nil, errors.New(n.Path() + " is not a directory")
	}
	s := newScanner(o)
	path := n.Path()
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	fresh := newNode(n.Name, n.Parent, path, fi)
	if n.Parent == nil {
		if fi, err = os.Stat(path); err != nil {
			return nil, err
		}
		fresh = newNode(n.Name, nil, path, fi)
		fresh.Kind = Dir
	}
	if fresh.Kind != Dir {
		return nil, errors.New(path + " is no longer a directory")
	}
	n.Children, n.Err = nil, nil
	n.Usage, n.Apparent, n.ModTime, n.ID, n.Links = fresh.Usage, fresh.Apparent, fresh.ModTime, fresh.ID, fresh.Links
	s.dir(ctx, n, path, n.ID.Dev)
	s.wg.Wait()
	return s.warnings, ctx.Err()
}

func (s *scanner) warn(path string, err error) {
	s.mu.Lock()
	s.warnings = append(s.warnings, Warning{path, err})
	s.mu.Unlock()
}

func (s *scanner) dir(ctx context.Context, n *Node, path string, dev uint64) {
	s.opts.Progress.Dirs.Add(1)
	entries, err := s.readDir(path)
	if err != nil {
		// os.ReadDir returns what it managed to read alongside the error.
		n.Err = err
		s.warn(path, err)
	}
	children := make([]*Node, 0, len(entries))
	for _, e := range entries {
		if ctx.Err() != nil {
			return
		}
		p := filepath.Join(path, e.Name())
		fi, err := s.lstat(p)
		if err != nil {
			s.warn(p, err) // vanished or unreadable between listing and stat
			continue
		}
		c := newNode(e.Name(), n, p, fi)
		children = append(children, c)
		s.opts.Progress.Items.Add(1)
		s.opts.Progress.Bytes.Add(c.Usage)
	}
	n.Children = children
	for _, c := range children {
		if c.Kind != Dir {
			continue
		}
		if s.opts.OneFS && c.ID.Valid() && c.ID.Dev != dev {
			continue
		}
		s.spawn(ctx, c, filepath.Join(path, c.Name), dev)
	}
}

// spawn reads a subdirectory on a new goroutine while there is a free worker
// slot, and on the current one otherwise, so the walk can never deadlock
// waiting for slots held by its own ancestors.
func (s *scanner) spawn(ctx context.Context, n *Node, path string, dev uint64) {
	select {
	case s.sem <- struct{}{}:
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer func() { <-s.sem }()
			s.dir(ctx, n, path, dev)
		}()
	default:
		s.dir(ctx, n, path, dev)
	}
}

func newNode(name string, parent *Node, path string, fi os.FileInfo) *Node {
	n := &Node{Name: name, Parent: parent, ModTime: fi.ModTime(), Kind: kindOf(fi)}
	switch n.Kind {
	case File:
		n.Apparent = fi.Size()
	case Symlink:
		if t, err := os.Readlink(path); err == nil {
			n.Target = t
			n.Apparent = int64(len(t))
		}
	case Dir:
		n.Apparent = dirApparent(fi)
	}
	fillStat(n, path, fi)
	return n
}

func kindOf(fi os.FileInfo) Kind {
	m := fi.Mode()
	switch {
	case m&os.ModeSymlink != 0:
		return Symlink
	case m.IsDir():
		return Dir
	case m.IsRegular():
		return File
	case isReparsePoint(fi):
		return Symlink
	default:
		return Other
	}
}

// KindOf classifies a fresh Lstat result the same way the scan did.
func KindOf(fi os.FileInfo) Kind { return kindOf(fi) }

// Identify returns the FileID for path, used to check that a file has not
// been swapped for another since the scan.
func Identify(path string, fi os.FileInfo) FileID {
	n := &Node{Kind: kindOf(fi)}
	fillStat(n, path, fi)
	return n.ID
}
