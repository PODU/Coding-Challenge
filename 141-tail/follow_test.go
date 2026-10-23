package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer is a bytes.Buffer safe to read while cctail writes to it.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// follower is a cctail -f running in the background of a test.
type follower struct {
	t        *testing.T
	dir      string
	out, err *syncBuffer
	cancel   context.CancelFunc
	done     chan int
}

func startFollow(t *testing.T, dir string, args ...string) *follower {
	t.Helper()
	cfg, err := parseArgs(args)
	if err != nil {
		t.Fatal(err)
	}
	for i, f := range cfg.files {
		cfg.files[i] = filepath.Join(dir, f)
	}
	ctx, cancel := context.WithCancel(context.Background())
	f := &follower{t: t, dir: dir, out: &syncBuffer{}, err: &syncBuffer{}, cancel: cancel, done: make(chan int, 1)}
	go func() { f.done <- run(ctx, cfg, nil, f.out, f.err) }()
	t.Cleanup(func() { f.stop() })
	return f
}

// clean strips the temp dir from paths so assertions can use bare names.
func (f *follower) clean(s string) string {
	return strings.ReplaceAll(s, f.dir+string(filepath.Separator), "")
}

// waitOut waits until stdout equals want, failing after timeout.
func (f *follower) waitOut(want string, timeout time.Duration) time.Duration {
	f.t.Helper()
	start := time.Now()
	for time.Since(start) < timeout {
		if f.clean(f.out.String()) == want {
			return time.Since(start)
		}
		time.Sleep(5 * time.Millisecond)
	}
	f.t.Fatalf("stdout = %q\nwant     %q\nstderr = %q", f.clean(f.out.String()), want, f.clean(f.err.String()))
	return 0
}

func (f *follower) stop() int {
	f.cancel()
	select {
	case code := <-f.done:
		f.done <- code // keep it for repeated calls
		return code
	case <-time.After(3 * time.Second):
		f.t.Fatal("cctail did not exit promptly after cancellation")
		return -1
	}
}

func appendFile(t *testing.T, path, s string) {
	t.Helper()
	fh, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	if _, err := fh.WriteString(s); err != nil {
		t.Fatal(err)
	}
}

const slowPoll = "5" // seconds: long enough that only a watcher wakes us fast

func TestFollowAppend(t *testing.T) {
	for _, mode := range []string{"watch", "poll"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			log := filepath.Join(dir, "test.log")
			writeFile(t, log, seqText(1, 15))

			args := []string{"-f", "-n", "2", "-s", slowPoll, "test.log"}
			if mode == "poll" {
				args = []string{"-f", "-n", "2", "-s", "0.05", "--poll", "test.log"}
			}
			f := startFollow(t, dir, args...)
			f.waitOut("14\n15\n", 2*time.Second)

			appendFile(t, log, "16\n")
			// With a 5s poll interval, seeing it well under a second proves
			// the OS notification path (step 9) is doing the work.
			if took := f.waitOut("14\n15\n16\n", 2*time.Second); mode == "watch" && took > time.Second {
				t.Errorf("watcher took %v to deliver an append", took)
			}
			appendFile(t, log, "17\n18\n")
			f.waitOut("14\n15\n16\n17\n18\n", 2*time.Second)

			if code := f.stop(); code != 130 {
				t.Errorf("exit code after cancel = %d, want 130", code)
			}
		})
	}
}

func TestFollowTruncation(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "test.log")
	writeFile(t, log, seqText(1, 15))
	f := startFollow(t, dir, "-f", "-n", "1", "-s", "0.05", "test.log")
	f.waitOut("15\n", 2*time.Second)

	writeFile(t, log, "") // > test.log
	time.Sleep(200 * time.Millisecond)
	appendFile(t, log, "fresh\n")
	f.waitOut("15\nfresh\n", 2*time.Second)
	if !strings.Contains(f.err.String(), "file truncated") {
		t.Errorf("stderr = %q, want a truncation notice", f.err.String())
	}
}

// rotate renames path aside and starts a new file with content at path.
func rotate(t *testing.T, path, content string) {
	t.Helper()
	if err := os.Rename(path, path+".old"); err != nil {
		t.Fatalf("rename while followed: %v", err)
	}
	time.Sleep(150 * time.Millisecond)
	writeFile(t, path, content)
}

func TestFollowNameSeesRotation(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "test.log")
	writeFile(t, log, seqText(1, 3))
	f := startFollow(t, dir, "-F", "-n", "1", "-s", "0.05", "test.log")
	f.waitOut("3\n", 2*time.Second)

	rotate(t, log, "new file line\n")
	f.waitOut("3\nnew file line\n", 2*time.Second)
	if !strings.Contains(f.err.String(), "following new file") {
		t.Errorf("stderr = %q, want a rotation notice", f.err.String())
	}
	appendFile(t, log, "more\n")
	f.waitOut("3\nnew file line\nmore\n", 2*time.Second)
}

func TestFollowDescriptorStaysOnOldFile(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "test.log")
	writeFile(t, log, seqText(1, 3))
	f := startFollow(t, dir, "-f", "-n", "1", "-s", "0.05", "test.log")
	f.waitOut("3\n", 2*time.Second)

	rotate(t, log, "new file line\n")
	// The renamed file is still what -f follows.
	appendFile(t, log+".old", "old file line\n")
	f.waitOut("3\nold file line\n", 2*time.Second)
	time.Sleep(300 * time.Millisecond)
	if got := f.clean(f.out.String()); strings.Contains(got, "new file line") {
		t.Errorf("-f picked up the replacement file: %q", got)
	}
}

func TestFollowNameWaitsForMissingFile(t *testing.T) {
	dir := t.TempDir()
	f := startFollow(t, dir, "-F", "-s", "0.05", "later.log")
	time.Sleep(150 * time.Millisecond)
	if !strings.Contains(f.err.String(), "cannot open") {
		t.Errorf("stderr = %q, want cannot-open warning", f.err.String())
	}
	writeFile(t, filepath.Join(dir, "later.log"), "hello\n")
	f.waitOut("hello\n", 2*time.Second)
}

func TestFollowMultipleFilesHeaders(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "test.log"), filepath.Join(dir, "test2.log")
	writeFile(t, a, seqText(1, 15))
	writeFile(t, b, seqText(100, 115))
	f := startFollow(t, dir, "-f", "-n", "1", "-s", "0.05", "test.log", "test2.log")
	want := "==> test.log <==\n15\n\n==> test2.log <==\n115\n"
	f.waitOut(want, 2*time.Second)

	appendFile(t, a, "16\n")
	want += "\n==> test.log <==\n16\n"
	f.waitOut(want, 2*time.Second)
	appendFile(t, a, "17\n") // same file again: no new header
	want += "17\n"
	f.waitOut(want, 2*time.Second)
	appendFile(t, b, "116\n")
	want += "\n==> test2.log <==\n116\n"
	f.waitOut(want, 2*time.Second)
}

func TestFollowQuietMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "test.log"), filepath.Join(dir, "test2.log")
	writeFile(t, a, seqText(1, 15))
	writeFile(t, b, seqText(100, 115))
	f := startFollow(t, dir, "-q", "-f", "-n", "1", "-s", "0.05", "test.log", "test2.log")
	f.waitOut("15\n115\n", 2*time.Second)
	appendFile(t, b, "116\n")
	f.waitOut("15\n115\n116\n", 2*time.Second)
	appendFile(t, a, "16\n")
	f.waitOut("15\n115\n116\n16\n", 2*time.Second)
}

func TestFollowStopsWhenPidExits(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// Re-run the test binary as a short-lived child process.
	cmd := exec.Command(exe, "-test.run=^$")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.log"), "x\n")
	f := startFollow(t, dir, "-f", "-s", "0.05", "--pid="+strconv.Itoa(cmd.Process.Pid), "test.log")
	cmd.Wait()
	select {
	case code := <-f.done:
		f.done <- code
		if code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("did not exit after --pid process died")
	}
}
