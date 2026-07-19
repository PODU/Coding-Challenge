// Package input handles key bindings. Bytes typed by the client pass straight
// through to the shell unless they follow the prefix key (Ctrl+b by default)
// or the pane is in copy mode. Bindings act on the session directly; anything
// the server has to do itself comes back in a Result.
package input

import (
	"cctmux/internal/layout"
	"cctmux/internal/session"
)

const DefaultPrefix = 0x02

// Result tells the server what to do after a key.
type Result struct {
	Forward []byte // send these to the active pane's shell
	Detach  bool
	Prompt  bool // open the command prompt (prefix+:)
}

// Dispatcher tracks the prefix state for one attached client.
type Dispatcher struct {
	Prefix byte
	s      *session.Session

	waiting bool   // the prefix was pressed; the next key is a command
	pending []byte // partial escape sequence
}

// New returns a Dispatcher for s. A prefix of 0 means Ctrl+b.
func New(s *session.Session, prefix byte) *Dispatcher {
	if prefix == 0 {
		prefix = DefaultPrefix
	}
	return &Dispatcher{Prefix: prefix, s: s}
}

func (d *Dispatcher) SetSession(s *session.Session) { d.s = s }

// Dispatch handles one byte of client input.
func (d *Dispatcher) Dispatch(b byte) Result {
	active := d.s.ActiveWindow().ActivePane()
	if active != nil && active.Copy {
		return d.copyMode(b)
	}
	if d.waiting {
		return d.command(b)
	}
	if b == d.Prefix {
		d.waiting = true
		return Result{}
	}
	return Result{Forward: []byte{b}}
}

// command handles the key after the prefix, which may be several bytes long.
func (d *Dispatcher) command(b byte) Result {
	// Pressing the prefix twice sends it through to the shell.
	if len(d.pending) == 0 && b == d.Prefix {
		d.waiting = false
		return Result{Forward: []byte{d.Prefix}}
	}
	d.pending = append(d.pending, b)
	tok, complete := decode(d.pending)
	if !complete {
		return Result{}
	}
	d.pending = nil
	d.waiting = false
	return d.runBinding(tok)
}

func (d *Dispatcher) runBinding(k key) Result {
	s := d.s
	switch {
	case k.special == arrowUp && k.ctrl:
		s.ResizePane(layout.Up, 1)
	case k.special == arrowDown && k.ctrl:
		s.ResizePane(layout.Down, 1)
	case k.special == arrowLeft && k.ctrl:
		s.ResizePane(layout.Left, 1)
	case k.special == arrowRight && k.ctrl:
		s.ResizePane(layout.Right, 1)
	case k.special == arrowUp:
		s.Navigate(layout.Up)
	case k.special == arrowDown:
		s.Navigate(layout.Down)
	case k.special == arrowLeft:
		s.Navigate(layout.Left)
	case k.special == arrowRight:
		s.Navigate(layout.Right)
	case k.r == 'd':
		return Result{Detach: true}
	case k.r == 'c':
		s.NewWindow("bash")
	case k.r == 'n':
		s.NextWindow()
	case k.r == 'p':
		s.PrevWindow()
	case k.r >= '0' && k.r <= '9':
		s.SelectWindow(int(k.r - '0'))
	case k.r == '"':
		s.SplitActive(layout.Horizontal) // top/bottom
	case k.r == '%':
		s.SplitActive(layout.Vertical) // left/right
	case k.r == 'z':
		s.ToggleZoom()
	case k.r == 'x':
		s.CloseActivePane()
	case k.r == '[':
		s.EnterCopyMode()
	case k.r == ':':
		return Result{Prompt: true}
	}
	return Result{}
}

func (d *Dispatcher) copyMode(b byte) Result {
	d.pending = append(d.pending, b)
	tok, complete := decode(d.pending)
	if !complete {
		return Result{}
	}
	d.pending = nil
	s := d.s
	switch {
	case tok.r == 'q':
		s.ExitCopyMode()
	case tok.special == arrowUp:
		s.Scroll(1)
	case tok.special == arrowDown:
		s.Scroll(-1)
	case tok.special == pageUp:
		s.Scroll(10)
	case tok.special == pageDown:
		s.Scroll(-10)
	}
	return Result{}
}

type special int

const (
	none special = iota
	arrowUp
	arrowDown
	arrowLeft
	arrowRight
	pageUp
	pageDown
)

type key struct {
	r       byte // the byte itself, for single-byte keys
	special special
	ctrl    bool
}

// decode reads one key from buf. It returns false if buf ends partway through
// an escape sequence and more bytes are needed.
func decode(buf []byte) (key, bool) {
	if len(buf) == 0 {
		return key{}, false
	}
	if buf[0] != 0x1b {
		return key{r: buf[0]}, true
	}
	if len(buf) < 2 {
		return key{}, false
	}
	if buf[1] != '[' {
		return key{r: 0x1b}, true
	}
	body := buf[2:]
	if len(body) == 0 {
		return key{}, false
	}
	last := body[len(body)-1]
	switch last {
	case 'A', 'B', 'C', 'D':
		ctrl := hasCtrlModifier(body[:len(body)-1])
		return key{special: arrowFor(last), ctrl: ctrl}, true
	case '~':
		switch string(body[:len(body)-1]) {
		case "5":
			return key{special: pageUp}, true
		case "6":
			return key{special: pageDown}, true
		}
		return key{special: none}, true
	}
	// Some other CSI sequence: done once we see its final byte.
	if last >= 0x40 && last <= 0x7e {
		return key{special: none}, true
	}
	return key{}, false
}

func arrowFor(final byte) special {
	switch final {
	case 'A':
		return arrowUp
	case 'B':
		return arrowDown
	case 'C':
		return arrowRight
	case 'D':
		return arrowLeft
	}
	return none
}

// hasCtrlModifier reports whether CSI parameters such as the "1;5" in
// ESC [ 1;5A include Ctrl.
func hasCtrlModifier(params []byte) bool {
	s := string(params)
	return s == "1;5" || s == "1;6" || s == "5"
}
