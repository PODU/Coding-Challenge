package main

import (
	"syscall"
	"unsafe"
)

// dirWatchWindows is one directory being watched with ReadDirectoryChangesW.
// The overlapped struct and buffer must stay at fixed addresses while the
// kernel owns them; Go's heap does not move objects, so a pointer suffices.
type dirWatchWindows struct {
	h   syscall.Handle
	ov  syscall.Overlapped
	buf [16 * 1024]byte
}

// rdcWatcher multiplexes overlapped ReadDirectoryChangesW calls on one I/O
// completion port. Directory watches report writes, size changes, creates
// and renames of the files inside, covering both -f and -F.
type rdcWatcher struct {
	port syscall.Handle
	dirs []*dirWatchWindows
	ch   chan struct{}
	done chan struct{}
}

const (
	rdcMask = syscall.FILE_NOTIFY_CHANGE_FILE_NAME |
		syscall.FILE_NOTIFY_CHANGE_SIZE |
		syscall.FILE_NOTIFY_CHANGE_LAST_WRITE
	// closeKey is posted to the port by Close to stop the loop.
	closeKey = ^uint32(0)
)

func newWatcher(paths []string) (watcher, error) {
	port, err := syscall.CreateIoCompletionPort(syscall.InvalidHandle, 0, 0, 1)
	if err != nil {
		return nil, err
	}
	w := &rdcWatcher{port: port, ch: make(chan struct{}, 1), done: make(chan struct{})}
	for _, d := range parentDirs(paths) {
		p, err := syscall.UTF16PtrFromString(d)
		if err != nil {
			continue
		}
		h, err := syscall.CreateFile(p, syscall.FILE_LIST_DIRECTORY,
			syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
			nil, syscall.OPEN_EXISTING,
			syscall.FILE_FLAG_BACKUP_SEMANTICS|syscall.FILE_FLAG_OVERLAPPED, 0)
		if err != nil {
			continue
		}
		key := uint32(len(w.dirs))
		if _, err := syscall.CreateIoCompletionPort(h, port, key, 0); err != nil {
			syscall.CloseHandle(h)
			continue
		}
		dw := &dirWatchWindows{h: h}
		if err := dw.arm(); err != nil {
			syscall.CloseHandle(h)
			continue
		}
		w.dirs = append(w.dirs, dw)
	}
	if len(w.dirs) == 0 {
		syscall.CloseHandle(port)
		return nil, syscall.ERROR_PATH_NOT_FOUND
	}
	go w.loop()
	return w, nil
}

// arm issues the next asynchronous ReadDirectoryChangesW on the directory.
func (d *dirWatchWindows) arm() error {
	d.ov = syscall.Overlapped{}
	return syscall.ReadDirectoryChanges(d.h, &d.buf[0], uint32(unsafe.Sizeof(d.buf)),
		false, rdcMask, nil, &d.ov, 0)
}

func (w *rdcWatcher) loop() {
	defer close(w.done)
	for {
		var (
			n   uint32
			key uint32
			ov  *syscall.Overlapped
		)
		err := syscall.GetQueuedCompletionStatus(w.port, &n, &key, &ov, syscall.INFINITE)
		if key == closeKey {
			return
		}
		if ov == nil {
			return // the port itself failed
		}
		if int(key) >= len(w.dirs) {
			continue
		}
		// Even an error (e.g. buffer overflow) means "something changed".
		notify(w.ch)
		if err == syscall.ERROR_OPERATION_ABORTED {
			continue
		}
		if w.dirs[key].arm() != nil {
			continue // directory gone; polling still covers it
		}
	}
}

func (w *rdcWatcher) Events() <-chan struct{} { return w.ch }
func (w *rdcWatcher) Refresh()                {}

// Close stops the loop first, so no buffer is re-armed, then cancels the
// outstanding reads and releases every handle.
func (w *rdcWatcher) Close() error {
	syscall.PostQueuedCompletionStatus(w.port, 0, closeKey, nil)
	<-w.done
	for _, d := range w.dirs {
		syscall.CancelIoEx(d.h, nil)
		syscall.CloseHandle(d.h)
	}
	return syscall.CloseHandle(w.port)
}
