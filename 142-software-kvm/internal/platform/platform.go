// Package platform is the boundary between the KVM logic and the operating
// system: capturing local input, injecting remote input, the cursor, the
// clipboard and the screensaver.
package platform

import "fmt"

// Rect is a screen rectangle in desktop pixels.
type Rect struct{ X, Y, W, H int }

// Center is the middle pixel of r.
func (r Rect) Center() (int, int) { return r.X + r.W/2, r.Y + r.H/2 }

// Clamp returns the point inside r closest to (x, y).
func (r Rect) Clamp(x, y int) (int, int) {
	return min(max(x, r.X), r.X+r.W-1), min(max(y, r.Y), r.Y+r.H-1)
}

func (r Rect) String() string { return fmt.Sprintf("%dx%d+%d+%d", r.W, r.H, r.X, r.Y) }

// EventKind says which fields of an Event are meaningful.
type EventKind int

const (
	// MouseMotion: the local cursor moved to (X, Y). Sent while not captured.
	MouseMotion EventKind = iota
	// MouseDelta: the mouse moved by (DX, DY) while captured. The local
	// cursor itself stays put.
	MouseDelta
	// MouseButton: Button went Down or up.
	MouseButton
	// MouseWheel: scrolled by (DX, DY) in WheelDelta units; +DY is up.
	MouseWheel
	// Key: a Barrier KeyID went Down or up (Repeat marks auto-repeat downs).
	// Mask is the modifier state including this key's own effect.
	Key
	// ClipboardChanged: another program changed clipboard ClipboardID.
	ClipboardChanged
	// ScreenSaver: the screensaver started (On) or stopped.
	ScreenSaver
	// ShapeChanged: the desktop was resized; call Shape again.
	ShapeChanged
)

// Event is one thing that happened on the local machine.
type Event struct {
	Kind        EventKind
	X, Y        int
	DX, DY      int
	Button      uint8
	Down        bool
	Repeat      bool
	KeyID       uint16
	Mask        uint16
	KeyButton   uint16
	ClipboardID uint8
	On          bool
}

// Screen is everything the server and client need from the local desktop.
// The server uses it as the primary screen (capture, local events); the
// client uses it as a secondary screen (input injection).
type Screen interface {
	Shape() Rect
	CursorPos() (x, y int)
	Events() <-chan Event

	// Capture(true) is called when the cursor leaves for a remote screen:
	// local input must stop reaching local programs and mouse motion must
	// arrive as MouseDelta. Capture(false) undoes it.
	Capture(on bool)

	// Enter and Leave bracket the time a secondary screen is active.
	Enter(x, y int, mask uint16)
	Leave()

	MouseMove(x, y int)
	MouseRelMove(dx, dy int)
	MouseButton(button uint8, down bool)
	MouseWheel(dx, dy int)
	KeyDown(id, mask, button uint16)
	KeyUp(id, mask, button uint16)
	KeyRepeat(id, mask, count, button uint16)

	ScreenSaver(on bool)

	// Clipboard returns the text on clipboard id. SetClipboard must not
	// produce a ClipboardChanged event for the text it set.
	Clipboard(id uint8) (string, bool)
	SetClipboard(id uint8, text string)

	Close() error
}
