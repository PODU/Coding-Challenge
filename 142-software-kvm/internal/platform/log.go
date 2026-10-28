package platform

import (
	"fmt"
	"io"
	"sync"
)

// Log is a Screen that prints every action instead of performing it and
// never produces input. It runs on any OS, which makes it useful for
// checking interoperability: point a genuine Barrier server at a client
// using it and watch the messages arrive.
type Log struct {
	mu     sync.Mutex
	w      io.Writer
	shape  Rect
	x, y   int
	clip   [2]string
	events chan Event
}

// NewLog creates a log screen of the given shape.
func NewLog(w io.Writer, shape Rect) *Log {
	l := &Log{w: w, shape: shape, events: make(chan Event)}
	l.x, l.y = shape.Center()
	return l
}

func (l *Log) printf(format string, a ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(l.w, format+"\n", a...)
}

func (l *Log) Shape() Rect { return l.shape }
func (l *Log) CursorPos() (int, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.x, l.y
}
func (l *Log) Events() <-chan Event { return l.events }
func (l *Log) Capture(on bool)      { l.printf("capture %v", on) }
func (l *Log) Enter(x, y int, mask uint16) {
	l.setPos(x, y)
	l.printf("enter at (%d,%d) modifiers=%#04x", x, y, mask)
}
func (l *Log) Leave() { l.printf("leave") }
func (l *Log) setPos(x, y int) {
	l.mu.Lock()
	l.x, l.y = l.shape.Clamp(x, y)
	l.mu.Unlock()
}
func (l *Log) MouseMove(x, y int) { l.setPos(x, y); l.printf("mouse move (%d,%d)", x, y) }
func (l *Log) MouseRelMove(dx, dy int) {
	x, y := l.CursorPos()
	l.setPos(x+dx, y+dy)
	l.printf("mouse move by (%+d,%+d)", dx, dy)
}
func (l *Log) MouseButton(b uint8, down bool) { l.printf("mouse button %d %s", b, upDown(down)) }
func (l *Log) MouseWheel(dx, dy int)          { l.printf("mouse wheel (%+d,%+d)", dx, dy) }
func (l *Log) KeyDown(id, mask, button uint16) {
	l.printf("key %s down modifiers=%#04x button=%d", keyName(id), mask, button)
}
func (l *Log) KeyUp(id, mask, button uint16) {
	l.printf("key %s up modifiers=%#04x button=%d", keyName(id), mask, button)
}
func (l *Log) KeyRepeat(id, mask, count, button uint16) {
	l.printf("key %s repeat x%d modifiers=%#04x button=%d", keyName(id), count, mask, button)
}
func (l *Log) ScreenSaver(on bool) { l.printf("screensaver %v", on) }
func (l *Log) Clipboard(id uint8) (string, bool) {
	if int(id) >= len(l.clip) {
		return "", false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.clip[id], true
}
func (l *Log) SetClipboard(id uint8, text string) {
	if int(id) < len(l.clip) {
		l.mu.Lock()
		l.clip[id] = text
		l.mu.Unlock()
	}
	l.printf("clipboard %d = %q", id, text)
}
func (l *Log) Close() error { return nil }

func upDown(down bool) string {
	if down {
		return "down"
	}
	return "up"
}

func keyName(id uint16) string {
	if id >= 0x20 && id < 0xE000 {
		return fmt.Sprintf("%q", rune(id))
	}
	return fmt.Sprintf("%#04x", id)
}
