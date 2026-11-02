package main

import (
	"context"
	"io"
	"math/rand/v2"
	"time"
)

// screen is the terminal as the animation loop sees it. Tests supply a fake.
type screen interface {
	io.Reader
	io.Writer
	Size() (w, h int, err error)
	Flush() error
}

// frameDelay maps the -u value to time per frame: 10ms per step, as in
// cmatrix. Zero still sleeps a little so the terminal can keep up.
func frameDelay(u int) time.Duration {
	if u <= 0 {
		return 2 * time.Millisecond
	}
	return time.Duration(u) * 10 * time.Millisecond
}

// run draws until the user quits or ctx is cancelled. resized may be nil;
// the size is also checked before every frame, which is what catches resizes
// on Windows, where there is no SIGWINCH.
func run(ctx context.Context, o Options, s screen, resized <-chan struct{}, rng *rand.Rand) (err error) {
	setup := escAltScreenOn + escHideCursor + escWrapOff + escReset + escClear
	if o.Screensaver {
		setup += escMouseOn
	}
	io.WriteString(s, setup)
	defer func() {
		teardown := escReset
		if o.Screensaver {
			teardown += escMouseOff
		}
		teardown += escWrapOn + escShowCursor + escAltScreenOff
		io.WriteString(s, teardown)
		if ferr := s.Flush(); err == nil {
			err = ferr
		}
	}()

	events := make(chan Event, 64)
	go readEvents(s, events)

	w, h, _ := s.Size()
	m := NewMatrix(w, h, newSimConfig(o), rng)
	var r Renderer
	delay := o.Delay
	timer := time.NewTimer(0)
	defer timer.Stop()

	checkSize := func() {
		nw, nh, err := s.Size()
		if err == nil && (nw != m.W || nh != m.H) {
			m.Resize(nw, nh)
			r.Invalidate()
		}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-events:
			if !ok {
				return nil // input closed: nothing can ever quit us, so stop now
			}
			a := decide(ev, o.Screensaver)
			if a.quit {
				return nil
			}
			if a.speed >= 0 && a.speed != delay {
				delay = a.speed
				timer.Reset(frameDelay(delay))
			}
		case <-resized:
			checkSize()
		case <-timer.C:
			checkSize()
			m.Step()
			if _, err := s.Write(r.Frame(m)); err != nil {
				return err
			}
			if err := s.Flush(); err != nil {
				return err
			}
			timer.Reset(frameDelay(delay))
		}
	}
}

func readEvents(r io.Reader, out chan<- Event) {
	defer close(out)
	buf := make([]byte, 256)
	for {
		n, err := r.Read(buf)
		for _, ev := range parseInput(buf[:n]) {
			out <- ev
		}
		if err != nil {
			return
		}
	}
}
