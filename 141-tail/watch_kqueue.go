//go:build darwin || freebsd || netbsd || openbsd || dragonfly

package main

import (
	"sync"
	"syscall"
)

// kqueueWatcher uses EVFILT_VNODE. Unlike inotify, a directory vnode only
// reports changes to its entries, not writes to files inside it, so each
// followed file is watched through its own descriptor as well, and those
// descriptors are re-opened by Refresh after rotation.
type kqueueWatcher struct {
	kq    int
	paths []string
	ch    chan struct{}

	mu  sync.Mutex
	fds []int // directory + file descriptors currently registered
}

func newWatcher(paths []string) (watcher, error) {
	kq, err := syscall.Kqueue()
	if err != nil {
		return nil, err
	}
	syscall.CloseOnExec(kq)
	w := &kqueueWatcher{kq: kq, paths: paths, ch: make(chan struct{}, 1)}
	w.Refresh()
	go w.loop()
	return w, nil
}

// Refresh drops every registration and re-registers the directories and the
// files currently found at the followed paths.
func (w *kqueueWatcher) Refresh() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, fd := range w.fds {
		syscall.Close(fd) // closing a descriptor removes its kevents
	}
	w.fds = w.fds[:0]
	for _, p := range append(parentDirs(w.paths), w.paths...) {
		fd, err := syscall.Open(p, syscall.O_RDONLY|syscall.O_CLOEXEC|oEvtOnly, 0)
		if err != nil {
			continue
		}
		var ev syscall.Kevent_t
		syscall.SetKevent(&ev, fd, syscall.EVFILT_VNODE, syscall.EV_ADD|syscall.EV_CLEAR)
		ev.Fflags = syscall.NOTE_WRITE | syscall.NOTE_EXTEND | syscall.NOTE_DELETE |
			syscall.NOTE_RENAME | syscall.NOTE_ATTRIB
		if _, err := syscall.Kevent(w.kq, []syscall.Kevent_t{ev}, nil, nil); err != nil {
			syscall.Close(fd)
			continue
		}
		w.fds = append(w.fds, fd)
	}
}

func (w *kqueueWatcher) loop() {
	events := make([]syscall.Kevent_t, 16)
	for {
		n, err := syscall.Kevent(w.kq, nil, events, nil)
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			return // kqueue closed
		}
		if n > 0 {
			notify(w.ch)
		}
	}
}

func (w *kqueueWatcher) Events() <-chan struct{} { return w.ch }

func (w *kqueueWatcher) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, fd := range w.fds {
		syscall.Close(fd)
	}
	w.fds = nil
	return syscall.Close(w.kq) // makes the blocked Kevent call return EBADF
}
