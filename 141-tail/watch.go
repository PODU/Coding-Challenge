package main

import "path/filepath"

// watcher delivers a wake-up whenever something may have changed for one of
// the followed paths. Wake-ups are coalesced and carry no detail: the follow
// loop re-stats every target anyway, so a spurious wake-up is harmless and a
// missed one is caught by the poll timer.
type watcher interface {
	Events() <-chan struct{}
	// Refresh re-arms watches after targets were reopened. Only backends that
	// watch open file descriptors (kqueue) need it.
	Refresh()
	Close() error
}

// parentDirs returns the distinct directories containing paths. Watching the
// directory rather than the file also reports creation and renames, which
// -F needs to spot rotation.
func parentDirs(paths []string) []string {
	seen := map[string]bool{}
	var dirs []string
	for _, p := range paths {
		d := filepath.Dir(p)
		if abs, err := filepath.Abs(d); err == nil {
			d = abs
		}
		if !seen[d] {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}
	return dirs
}

// notify sends a coalesced wake-up without ever blocking.
func notify(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}
