package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"syscall"
	"time"
)

// printer writes file output, inserting a '==> name <==' header whenever the
// file whose data is being written changes.
type printer struct {
	w       *bufio.Writer
	headers bool
	last    *target
	any     bool // something has been written, so headers get a blank line first
	err     error
}

func newPrinter(w io.Writer, headers bool) *printer {
	return &printer{w: bufio.NewWriterSize(w, chunkSize), headers: headers}
}

func (p *printer) header(t *target) {
	if !p.headers || p.last == t {
		return
	}
	if p.any {
		p.w.WriteString("\n")
	}
	fmt.Fprintf(p.w, "==> %s <==\n", t.name)
	p.last, p.any = t, true
}

// Write lets the printer be the destination of the tail functions; it records
// the first write error so a closed pipe can stop the program.
func (p *printer) Write(b []byte) (int, error) {
	if len(b) > 0 {
		p.any = true
	}
	n, err := p.w.Write(b)
	if err != nil && p.err == nil {
		p.err = err
	}
	return n, err
}

func (p *printer) flush() {
	if err := p.w.Flush(); err != nil && p.err == nil {
		p.err = err
	}
}

// target is one FILE operand and, while following, its open handle.
type target struct {
	name    string // as given on the command line, used in headers/messages
	path    string // "" for standard input
	f       *os.File
	info    os.FileInfo // identity of f, for spotting replacement under -F
	off     int64       // how far into f has been output
	stream  bool        // not a regular file: read until EOF, never seek
	missing bool        // -F: the path is currently absent (already reported)
	done    bool        // nothing more will ever be read (closed pipe, failed open)
}

// tailer runs the whole program over every target.
type tailer struct {
	cfg     *config
	stdin   *os.File
	out     *printer
	stderr  io.Writer
	targets []*target
	failed  bool
	refresh func() // re-arms watchers that track open files (kqueue)
}

func (t *tailer) warn(format string, a ...any) {
	t.out.flush() // keep stdout and stderr in a sensible order on a terminal
	fmt.Fprintf(t.stderr, "cctail: "+format+"\n", a...)
}

func (t *tailer) run(ctx context.Context) int {
	for _, name := range t.cfg.files {
		tg := &target{name: name, path: name}
		if name == "-" {
			tg.name, tg.path = "standard input", ""
		}
		t.targets = append(t.targets, tg)
		t.initial(tg)
		if t.out.err != nil {
			return 1
		}
	}
	t.out.flush()

	if t.cfg.follow {
		if code := t.follow(ctx); code != 0 {
			return code
		}
	}
	t.out.flush()
	if t.out.err != nil || t.failed {
		return 1
	}
	return 0
}

// initial opens tg and prints its tail, leaving it positioned for following.
func (t *tailer) initial(tg *target) {
	if tg.path == "" {
		tg.f = t.stdin
	} else {
		f, err := openShared(tg.path)
		if err != nil {
			t.warn("cannot open '%s' for reading: %s", tg.name, errText(err))
			t.failed = true
			tg.missing = true
			tg.done = !(t.cfg.follow && t.cfg.retry)
			return
		}
		tg.f = f
	}
	info, err := tg.f.Stat()
	if err != nil {
		t.warn("cannot fstat '%s': %s", tg.name, errText(err))
		t.closeTarget(tg)
		t.failed, tg.done = true, true
		return
	}
	if info.IsDir() {
		t.warn("error reading '%s': Is a directory", tg.name)
		t.closeTarget(tg)
		t.failed, tg.done = true, true
		return
	}
	tg.info = info
	tg.stream = !info.Mode().IsRegular()

	t.out.header(tg)
	// Files that report size 0 (e.g. under /proc) may still have content, so
	// they are read as streams too.
	seekable := !tg.stream && info.Size() > 0
	if err := t.printTail(tg, info.Size(), seekable); err != nil && t.out.err == nil {
		t.warn("error reading '%s': %s", tg.name, errText(err))
		t.failed = true
	}

	// A pipe or terminal cannot grow once drained (GNU tail likewise ignores
	// -f on a pipe), so stop tracking it; regular files stay open to follow.
	if !t.cfg.follow || tg.stream {
		t.closeTarget(tg)
		tg.done = true
	}
}

// printTail writes the requested tail of tg and records tg.off.
func (t *tailer) printTail(tg *target, size int64, seekable bool) error {
	cfg, f := t.cfg, tg.f
	var err error
	switch {
	case !seekable && cfg.fromStart && cfg.unit == unitLines:
		tg.off, err = fromLine(f, cfg.count, t.out)
	case !seekable && cfg.fromStart:
		tg.off, err = fromByte(f, cfg.count, t.out)
	case !seekable && cfg.unit == unitLines:
		tg.off, err = streamLastLines(f, cfg.count, t.out)
	case !seekable:
		tg.off, err = streamLastBytes(f, cfg.count, t.out)

	// Seekable: jump straight to the part that is needed.
	case cfg.fromStart && cfg.unit == unitLines:
		tg.off, err = fromLine(io.NewSectionReader(f, 0, size), cfg.count, t.out)
	default:
		var start int64
		switch {
		case cfg.fromStart:
			start = min(size, max(0, cfg.count-1))
		case cfg.unit == unitLines:
			start, err = lastLinesStart(f, size, cfg.count)
		default:
			start = lastBytesStart(size, cfg.count)
		}
		if err == nil {
			err = copyRange(f, start, size, t.out)
		}
		tg.off = size
	}
	return err
}

func (t *tailer) closeTarget(tg *target) {
	if tg.f != nil && tg.f != t.stdin {
		tg.f.Close()
	}
	tg.f = nil
}

// follow keeps printing data appended to the targets until ctx is cancelled,
// the --pid process exits, or nothing is left to follow. It wakes on OS
// file-change notifications when available and on a poll timer regardless,
// so a missed or unsupported notification only costs latency.
func (t *tailer) follow(ctx context.Context) int {
	defer func() {
		for _, tg := range t.targets {
			t.closeTarget(tg)
		}
	}()

	var paths []string
	for _, tg := range t.targets {
		if tg.path != "" && !tg.done {
			paths = append(paths, tg.path)
		}
	}
	if !t.anyLive() {
		if !t.failed {
			return 0 // only pipes, which -f ignores once drained
		}
		t.warn("no files remaining")
		return 1
	}

	var events <-chan struct{}
	if !t.cfg.poll && len(paths) > 0 {
		w, err := newWatcher(paths)
		if err != nil {
			t.warn("file-change notifications unavailable (%v); reverting to polling", err)
		} else {
			defer w.Close()
			events = w.Events()
			t.refresh = w.Refresh
		}
	}

	interval := max(t.cfg.interval, 10*time.Millisecond)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		// Check the writer first so data written just before it died is
		// still printed by the poll below.
		pidGone := t.cfg.pid > 0 && !processAlive(t.cfg.pid)

		reopened := false
		for _, tg := range t.targets {
			if !tg.done && t.poll(tg) {
				reopened = true
			}
		}
		t.out.flush()
		if t.out.err != nil {
			return 1 // stdout went away (e.g. piped into head)
		}
		if reopened && t.refresh != nil {
			t.refresh()
		}
		if pidGone {
			return 0
		}
		if !t.anyLive() {
			t.warn("no files remaining")
			return 1
		}

		select {
		case <-ctx.Done():
			return 130 // conventional status for termination by Ctrl+C
		case <-events:
		case <-ticker.C:
		}
	}
}

func (t *tailer) anyLive() bool {
	for _, tg := range t.targets {
		if !tg.done {
			return true
		}
	}
	return false
}

// poll prints anything new in tg and, under -F, notices the path being
// deleted, recreated, or replaced. It reports whether tg was (re)opened or
// closed, so watchers that track open files can be refreshed.
func (t *tailer) poll(tg *target) (changed bool) {
	if tg.f != nil {
		t.drain(tg)
	}
	if !t.cfg.followName || tg.path == "" {
		if tg.f == nil && !t.cfg.retry {
			tg.done = true
		}
		if tg.f == nil && t.cfg.retry {
			return t.reopen(tg, "has appeared; following new file")
		}
		return false
	}

	fi, err := os.Stat(tg.path)
	switch {
	case err != nil:
		if tg.f != nil {
			t.warn("'%s' has become inaccessible: %s", tg.name, errText(err))
			t.closeTarget(tg)
			tg.missing = true
			return true
		}
	case tg.f == nil:
		return t.reopen(tg, "has appeared; following new file")
	case !os.SameFile(fi, tg.info):
		t.closeTarget(tg)
		return t.reopen(tg, "has been replaced; following new file")
	}
	return false
}

// reopen opens tg.path afresh and prints it from the beginning.
func (t *tailer) reopen(tg *target, msg string) bool {
	f, err := openShared(tg.path)
	if err != nil {
		if !tg.missing {
			t.warn("cannot open '%s' for reading: %s", tg.name, errText(err))
			tg.missing = true
		}
		return false
	}
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		f.Close()
		return false
	}
	t.warn("'%s' %s", tg.name, msg)
	tg.f, tg.info, tg.off, tg.missing = f, info, 0, false
	tg.stream = !info.Mode().IsRegular()
	t.drain(tg)
	return true
}

// drain prints everything appended to tg since the last call. A file that is
// now shorter than what has been printed was truncated: start over from 0.
func (t *tailer) drain(tg *target) {
	if !tg.stream {
		info, err := tg.f.Stat()
		if err != nil {
			return
		}
		if info.Size() < tg.off {
			t.warn("%s: file truncated", tg.name)
			tg.off = 0
		}
		if info.Size() == tg.off {
			return
		}
	}
	buf := make([]byte, chunkSize)
	for {
		n, err := tg.f.ReadAt(buf, tg.off)
		if n > 0 {
			t.out.header(tg)
			t.out.Write(buf[:n])
			tg.off += int64(n)
		}
		if err != nil || n == 0 {
			return
		}
	}
}

// errText renders an error the way coreutils does: just the OS reason.
func errText(err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		err = pe.Err
	}
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "No such file or directory"
	case errors.Is(err, fs.ErrPermission):
		return "Permission denied"
	case errors.Is(err, syscall.EISDIR):
		return "Is a directory"
	}
	return err.Error()
}
