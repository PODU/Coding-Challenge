package main

import (
	"os"
	"syscall"
)

// inotifyWatcher watches the parent directory of each followed file. A
// directory watch reports writes to the files inside it as well as creates,
// renames and deletes, so one watch covers -f and -F.
type inotifyWatcher struct {
	f  *os.File
	ch chan struct{}
}

func newWatcher(paths []string) (watcher, error) {
	fd, err := syscall.InotifyInit1(syscall.IN_CLOEXEC | syscall.IN_NONBLOCK)
	if err != nil {
		return nil, err
	}
	const mask = syscall.IN_MODIFY | syscall.IN_ATTRIB | syscall.IN_CREATE |
		syscall.IN_DELETE | syscall.IN_MOVED_FROM | syscall.IN_MOVED_TO |
		syscall.IN_DELETE_SELF | syscall.IN_MOVE_SELF
	added := 0
	for _, d := range parentDirs(paths) {
		if _, err := syscall.InotifyAddWatch(fd, d, mask); err == nil {
			added++
		}
	}
	if added == 0 {
		syscall.Close(fd)
		return nil, syscall.ENOENT
	}
	// A non-blocking fd wrapped in os.File goes through Go's poller, so
	// Close unblocks the pending Read and the goroutine exits cleanly.
	w := &inotifyWatcher{f: os.NewFile(uintptr(fd), "inotify"), ch: make(chan struct{}, 1)}
	go w.loop()
	return w, nil
}

func (w *inotifyWatcher) loop() {
	buf := make([]byte, 64*(syscall.SizeofInotifyEvent+syscall.NAME_MAX+1))
	for {
		if _, err := w.f.Read(buf); err != nil {
			return
		}
		notify(w.ch)
	}
}

func (w *inotifyWatcher) Events() <-chan struct{} { return w.ch }
func (w *inotifyWatcher) Refresh()                {}
func (w *inotifyWatcher) Close() error            { return w.f.Close() }
