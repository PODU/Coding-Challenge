package main

import (
	"bytes"
	"context"
	"io"
	"math/rand/v2"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeScreen is a terminal whose keyboard is a pipe and whose size can be
// changed while the animation runs.
type fakeScreen struct {
	keys *io.PipeReader
	mu   sync.Mutex
	out  bytes.Buffer
	w, h int
}

func newFakeScreen(w, h int) (*fakeScreen, *io.PipeWriter) {
	pr, pw := io.Pipe()
	return &fakeScreen{keys: pr, w: w, h: h}, pw
}

func (f *fakeScreen) Read(p []byte) (int, error) { return f.keys.Read(p) }
func (f *fakeScreen) Flush() error               { return nil }
func (f *fakeScreen) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.out.Write(p)
}
func (f *fakeScreen) Size() (int, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.w, f.h, nil
}
func (f *fakeScreen) resize(w, h int) {
	f.mu.Lock()
	f.w, f.h = w, h
	f.mu.Unlock()
}
func (f *fakeScreen) output() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.out.String()
}

func startRun(t *testing.T, o Options, s *fakeScreen) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- run(context.Background(), o, s, nil, rand.New(rand.NewPCG(3, 4))) }()
	return done
}

func waitDone(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("run did not exit")
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func assertRestored(t *testing.T, out string, mouse bool) {
	t.Helper()
	want := escReset
	if mouse {
		want += escMouseOff
	}
	want += escWrapOn + escShowCursor + escAltScreenOff
	if !strings.HasSuffix(out, want) {
		t.Fatalf("terminal not restored; output ends %q", out[max(0, len(out)-80):])
	}
	if !strings.HasPrefix(out, escAltScreenOn+escHideCursor) {
		t.Fatalf("should switch to the alternate screen and hide the cursor first: %q", out[:min(len(out), 40)])
	}
}

func TestQuitsOnQAndIgnoresOtherKeys(t *testing.T) {
	s, keys := newFakeScreen(40, 12)
	done := startRun(t, Options{Delay: 0, Colour: colGreen}, s)
	waitFor(t, func() bool { return strings.Contains(s.output(), "\x1b[0;1;97m") })
	keys.Write([]byte("ax \r\x1b[A"))
	select {
	case <-done:
		t.Fatal("other keys should not quit")
	case <-time.After(100 * time.Millisecond):
	}
	keys.Write([]byte("q"))
	waitDone(t, done)
	assertRestored(t, s.output(), false)
}

func TestScreensaverQuitsOnAnyKeyOrMouse(t *testing.T) {
	for _, in := range []string{" ", "a", "5", "\r", "\x1b[<0;3;4M"} {
		s, keys := newFakeScreen(20, 8)
		done := startRun(t, Options{Screensaver: true, Delay: 0, Colour: colGreen}, s)
		keys.Write([]byte(in))
		waitDone(t, done)
		out := s.output()
		assertRestored(t, out, true)
		if !strings.Contains(out, escMouseOn) {
			t.Fatal("screensaver mode should enable mouse reporting")
		}
	}
}

func TestSpeedKeysChangeRate(t *testing.T) {
	count := func(out string) int { return strings.Count(out, "\x1b[0;1;97m") }
	s, keys := newFakeScreen(40, 12)
	done := startRun(t, Options{Delay: 10, Colour: colGreen}, s)
	time.Sleep(300 * time.Millisecond)
	slow := count(s.output())
	keys.Write([]byte("0"))
	before := len(s.output())
	time.Sleep(300 * time.Millisecond)
	fast := count(s.output()[before:])
	keys.Write([]byte("q"))
	waitDone(t, done)
	if fast <= slow*2 {
		t.Fatalf("pressing 0 should speed up: %d heads drawn slow vs %d fast", slow, fast)
	}
}

func TestResizeRedrawsWithinBounds(t *testing.T) {
	s, keys := newFakeScreen(40, 12)
	done := startRun(t, Options{Delay: 0, Colour: colGreen}, s)
	waitFor(t, func() bool { return len(s.output()) > 2000 })
	s.resize(10, 5)
	waitFor(t, func() bool { return strings.Count(s.output(), escClear) >= 3 })
	mark := len(s.output())
	time.Sleep(100 * time.Millisecond)
	tail := s.output()[mark:]
	keys.Write([]byte("q"))
	waitDone(t, done)
	// Every cursor move after the resize must land inside the 10x5 window.
	for _, m := range cursorMove.FindAllStringSubmatch(tail, -1) {
		row, _ := strconv.Atoi(m[1])
		col, _ := strconv.Atoi(m[2])
		if row > 5 || col > 10 {
			t.Fatalf("drew at %d,%d outside a 10x5 window", row, col)
		}
	}
}

var cursorMove = regexp.MustCompile(`\[(\d+);(\d+)H`)

func TestContextCancelRestores(t *testing.T) {
	s, _ := newFakeScreen(20, 8)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, Options{Delay: 4, Colour: colGreen}, s, nil, rand.New(rand.NewPCG(1, 1))) }()
	time.Sleep(50 * time.Millisecond)
	cancel()
	waitDone(t, done)
	assertRestored(t, s.output(), false)
}
