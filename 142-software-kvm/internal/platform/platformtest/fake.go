// Package platformtest provides an in-memory platform.Screen for tests.
package platformtest

import (
	"fmt"
	. "github.com/podu2997/cckvm/internal/platform"
	"strings"
	"sync"
	"testing"
	"time"
)

// Fake is an in-memory Screen for tests. Every output call is recorded as
// a line such as "move 10 20" or "keydown 97 1 30", and tests feed input
// with Emit.
type Fake struct {
	mu       sync.Mutex
	shape    Rect
	x, y     int
	captured bool
	clip     [2]string
	hasClip  [2]bool
	events   chan Event
	calls    chan string
}

// NewFake creates a fake screen of the given shape with the cursor at its
// center.
func NewFake(shape Rect) *Fake {
	f := &Fake{shape: shape, events: make(chan Event, 256), calls: make(chan string, 4096)}
	f.x, f.y = shape.Center()
	return f
}

func (f *Fake) record(format string, a ...any) {
	select {
	case f.calls <- fmt.Sprintf(format, a...):
	default:
	}
}

// Emit injects a local input event.
func (f *Fake) Emit(e Event) { f.events <- e }

// SetShape changes the reported shape (emit ShapeChanged to announce it).
func (f *Fake) SetShape(r Rect) { f.mu.Lock(); f.shape = r; f.mu.Unlock() }

// Captured reports the last Capture call.
func (f *Fake) Captured() bool { f.mu.Lock(); defer f.mu.Unlock(); return f.captured }

// Expect waits for the next recorded call starting with prefix, skipping
// others, and returns it.
func (f *Fake) Expect(t testing.TB, prefix string) string {
	t.Helper()
	deadline := time.After(3 * time.Second)
	var seen []string
	for {
		select {
		case c := <-f.calls:
			if strings.HasPrefix(c, prefix) {
				return c
			}
			seen = append(seen, c)
		case <-deadline:
			t.Fatalf("timed out waiting for %q; saw %q", prefix, seen)
			return ""
		}
	}
}

// ExpectNone fails if a call starting with prefix arrives within d.
func (f *Fake) ExpectNone(t testing.TB, prefix string, d time.Duration) {
	t.Helper()
	deadline := time.After(d)
	for {
		select {
		case c := <-f.calls:
			if strings.HasPrefix(c, prefix) {
				t.Fatalf("unexpected %q", c)
			}
		case <-deadline:
			return
		}
	}
}

// Drain discards recorded calls.
func (f *Fake) Drain() {
	for {
		select {
		case <-f.calls:
		default:
			return
		}
	}
}

func (f *Fake) Shape() Rect           { f.mu.Lock(); defer f.mu.Unlock(); return f.shape }
func (f *Fake) CursorPos() (int, int) { f.mu.Lock(); defer f.mu.Unlock(); return f.x, f.y }
func (f *Fake) Events() <-chan Event  { return f.events }

func (f *Fake) Capture(on bool) {
	f.mu.Lock()
	f.captured = on
	f.mu.Unlock()
	f.record("capture %v", on)
}

func (f *Fake) Enter(x, y int, mask uint16) {
	f.mu.Lock()
	f.x, f.y = x, y
	f.mu.Unlock()
	f.record("enter %d %d %d", x, y, mask)
}

func (f *Fake) Leave() { f.record("leave") }

func (f *Fake) MouseMove(x, y int) {
	f.mu.Lock()
	f.x, f.y = f.shape.Clamp(x, y)
	f.mu.Unlock()
	f.record("move %d %d", x, y)
}

func (f *Fake) MouseRelMove(dx, dy int) {
	f.mu.Lock()
	f.x, f.y = f.shape.Clamp(f.x+dx, f.y+dy)
	f.mu.Unlock()
	f.record("relmove %d %d", dx, dy)
}

func (f *Fake) MouseButton(b uint8, down bool) { f.record("button %d %v", b, down) }
func (f *Fake) MouseWheel(dx, dy int)          { f.record("wheel %d %d", dx, dy) }
func (f *Fake) KeyDown(id, mask, button uint16) {
	f.record("keydown %d %d %d", id, mask, button)
}
func (f *Fake) KeyUp(id, mask, button uint16) { f.record("keyup %d %d %d", id, mask, button) }
func (f *Fake) KeyRepeat(id, mask, count, button uint16) {
	f.record("keyrepeat %d %d %d %d", id, mask, count, button)
}
func (f *Fake) ScreenSaver(on bool) { f.record("screensaver %v", on) }

func (f *Fake) Clipboard(id uint8) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if int(id) >= len(f.clip) {
		return "", false
	}
	return f.clip[id], f.hasClip[id]
}

func (f *Fake) SetClipboard(id uint8, text string) {
	f.mu.Lock()
	if int(id) < len(f.clip) {
		f.clip[id], f.hasClip[id] = text, true
	}
	f.mu.Unlock()
	f.record("setclip %d %s", id, text)
}

// CopyText simulates a local program copying text: it updates the
// clipboard and emits ClipboardChanged.
func (f *Fake) CopyText(id uint8, text string) {
	f.mu.Lock()
	f.clip[id], f.hasClip[id] = text, true
	f.mu.Unlock()
	f.Emit(Event{Kind: ClipboardChanged, ClipboardID: id})
}

func (f *Fake) Close() error { return nil }
