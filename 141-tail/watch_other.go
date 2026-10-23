//go:build !linux && !windows && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly

package main

import "errors"

// newWatcher has no OS backend here; the follow loop falls back to polling.
func newWatcher([]string) (watcher, error) {
	return nil, errors.New("not supported on this platform")
}
