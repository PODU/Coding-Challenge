// Package term owns the terminal: raw mode, screen output, and turning the
// byte stream from the keyboard into key events.
//
// Input never comes from standard input - that is where the item list arrives
// from - so the controlling terminal is opened directly (/dev/tty, or the
// console devices on Windows).
package term

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"
)

// Key is one keypress. Name is always set and is the canonical name used by
// --expect ("ctrl-x", "enter", "up", "a"); Rune is non-zero for keys that
// produce a character.
type Key struct {
	Name string
	Rune rune
}

func (k Key) Printable() bool { return k.Rune != 0 }

func runeKey(r rune) Key {
	if r == ' ' {
		return Key{Name: "space", Rune: r}
	}
	return Key{Name: string(r), Rune: r}
}

type Terminal struct {
	in    *os.File
	out   *os.File
	w     *bufio.Writer
	state *state
	raw   bool

	// bytes is fed by the single goroutine that owns the input side. Both the
	// cursor-position query and the key decoder read from it, so neither can
	// swallow bytes meant for the other.
	bytes chan byte
}

// Open connects to the controlling terminal.
func Open() (*Terminal, error) {
	in, out, err := openTTY()
	if err != nil {
		return nil, err
	}
	return &Terminal{
		in:  in,
		out: out,
		w:   bufio.NewWriterSize(out, 64*1024),
	}, nil
}

func (t *Terminal) MakeRaw() error {
	st, err := makeRaw(t.in, t.out)
	if err != nil {
		return err
	}
	t.state = st
	t.raw = true
	return nil
}

func (t *Terminal) Restore() {
	if t.state != nil {
		restore(t.in, t.out, t.state)
		t.state = nil
		t.raw = false
	}
}

func (t *Terminal) Close() {
	t.Restore()
	if t.in != nil {
		t.in.Close()
	}
	if t.out != nil && t.out != t.in {
		t.out.Close()
	}
}

// Size returns the current window size, falling back to a sane default when
// the terminal cannot answer.
func (t *Terminal) Size() (cols, rows int) {
	c, r, err := size(t.out)
	if err != nil || c <= 0 || r <= 0 {
		return 80, 24
	}
	return c, r
}

func (t *Terminal) Write(s string)            { t.w.WriteString(s) }
func (t *Terminal) Writef(f string, a ...any) { fmt.Fprintf(t.w, f, a...) }
func (t *Terminal) Flush()                    { t.w.Flush() }
func (t *Terminal) MoveTo(row, col int)       { t.Writef("\x1b[%d;%dH", row, col) }
func (t *Terminal) ClearLine()                { t.Write("\x1b[2K") }
func (t *Terminal) ClearToEndOfScreen()       { t.Write("\x1b[J") }
func (t *Terminal) ClearScreen()              { t.Write("\x1b[2J\x1b[H") }
func (t *Terminal) HideCursor()               { t.Write("\x1b[?25l") }
func (t *Terminal) ShowCursor()               { t.Write("\x1b[?25h") }
func (t *Terminal) EnterAltScreen()           { t.Write("\x1b[?1049h") }
func (t *Terminal) ExitAltScreen()            { t.Write("\x1b[?1049l") }
func (t *Terminal) SGR(code string)           { t.Write("\x1b[" + code + "m") }
func (t *Terminal) Reset()                    { t.Write("\x1b[0m") }

// CursorPosition asks the terminal where the cursor is, so that --height can
// draw below whatever the shell has already printed.
//
// A terminal that does not answer must not wedge the program, so the wait is
// bounded and the caller falls back to the bottom of the screen.
func (t *Terminal) CursorPosition() (row, col int, err error) {
	t.startReader()
	t.Write("\x1b[6n")
	t.Flush()

	deadline := time.After(cursorReportTimeout)
	var buf []byte
	for {
		select {
		case b, ok := <-t.bytes:
			if !ok {
				return 0, 0, io.ErrUnexpectedEOF
			}
			buf = append(buf, b)
			if b != 'R' {
				if len(buf) > 64 {
					return 0, 0, io.ErrUnexpectedEOF
				}
				continue
			}
			// Reply: ESC [ row ; col R, possibly behind a keystroke that
			// arrived first.
			s := string(buf)
			i := strings.LastIndex(s, "\x1b[")
			if i < 0 {
				return 0, 0, io.ErrUnexpectedEOF
			}
			if _, err := fmt.Sscanf(s[i+2:], "%d;%d", &row, &col); err != nil {
				return 0, 0, err
			}
			return row, col, nil
		case <-deadline:
			return 0, 0, errNoCursorReport
		}
	}
}

var errNoCursorReport = errors.New("terminal did not report the cursor position")

const (
	// escTimeout is how long an incomplete escape sequence waits for its
	// remaining bytes before being reported as a bare Escape keypress.
	escTimeout = 40 * time.Millisecond
	// cursorReportTimeout bounds the wait for a terminal that will never
	// answer a cursor-position query.
	cursorReportTimeout = 300 * time.Millisecond
)

// startReader gives the input side of the terminal a single owner.
func (t *Terminal) startReader() {
	if t.bytes != nil {
		return
	}
	t.bytes = make(chan byte, 4096)
	go func() {
		defer close(t.bytes)
		buf := make([]byte, 256)
		for {
			n, err := t.in.Read(buf)
			for _, b := range buf[:n] {
				t.bytes <- b
			}
			if err != nil {
				return
			}
		}
	}()
}

// Events starts decoding keys and returns the key stream. The channel is
// closed when the terminal is closed.
func (t *Terminal) Events() <-chan Key {
	t.startReader()
	keys := make(chan Key, 256)
	go decode(t.bytes, keys)
	return keys
}

func decode(raw <-chan byte, keys chan<- Key) {
	defer close(keys)
	var buf []byte
	for {
		if len(buf) == 0 {
			b, ok := <-raw
			if !ok {
				return
			}
			buf = append(buf, b)
		}
		key, n, more := decodeOne(buf)
		if more {
			// Incomplete: wait briefly for the rest. A lone ESC is a real key,
			// so the wait has to end.
			select {
			case b, ok := <-raw:
				if !ok {
					return
				}
				buf = append(buf, b)
				continue
			case <-time.After(escTimeout):
				keys <- Key{Name: "esc"}
				buf = buf[1:]
				continue
			}
		}
		keys <- key
		buf = buf[n:]
	}
}

var csiKeys = map[string]string{
	"A": "up", "B": "down", "C": "right", "D": "left",
	"H": "home", "F": "end", "Z": "shift-tab",
	"1~": "home", "2~": "insert", "3~": "delete", "4~": "end",
	"5~": "pgup", "6~": "pgdn", "7~": "home", "8~": "end",
	"11~": "f1", "12~": "f2", "13~": "f3", "14~": "f4", "15~": "f5",
	"17~": "f6", "18~": "f7", "19~": "f8", "20~": "f9", "21~": "f10",
	"23~": "f11", "24~": "f12",
}

var ss3Keys = map[byte]string{'P': "f1", 'Q': "f2", 'R': "f3", 'S': "f4"}

// decodeOne decodes the first key in buf. It returns more=true when buf holds
// the start of a sequence whose remaining bytes have not arrived yet.
func decodeOne(buf []byte) (key Key, n int, more bool) {
	b := buf[0]
	switch {
	case b == 0x1b:
		return decodeEscape(buf)
	case b == 0x7f, b == 0x08:
		return Key{Name: "bspace"}, 1, false
	case b == 0x09:
		return Key{Name: "tab"}, 1, false
	case b == 0x0d:
		return Key{Name: "enter"}, 1, false
	case b == 0x00:
		return Key{Name: "ctrl-space"}, 1, false
	case b < 0x20:
		return Key{Name: "ctrl-" + string(rune('a'+b-1))}, 1, false
	case b < utf8.RuneSelf:
		return runeKey(rune(b)), 1, false
	}
	r, size := utf8.DecodeRune(buf)
	if r == utf8.RuneError && size <= 1 {
		if len(buf) < utf8.UTFMax {
			return Key{}, 0, true // possibly a truncated multi-byte rune
		}
		return runeKey(rune(b)), 1, false
	}
	return runeKey(r), size, false
}

func decodeEscape(buf []byte) (Key, int, bool) {
	if len(buf) == 1 {
		return Key{}, 0, true
	}
	switch buf[1] {
	case 0x1b:
		return Key{Name: "esc"}, 2, false
	case '[':
		return decodeCSI(buf)
	case 'O':
		if len(buf) < 3 {
			return Key{}, 0, true
		}
		if name, ok := ss3Keys[buf[2]]; ok {
			return Key{Name: name}, 3, false
		}
		return Key{Name: "esc"}, 1, false
	}
	// Alt + something.
	r, size := utf8.DecodeRune(buf[1:])
	if r == utf8.RuneError && size <= 1 {
		if len(buf) < 1+utf8.UTFMax {
			return Key{}, 0, true
		}
		return Key{Name: "esc"}, 1, false
	}
	if r == 0x0d {
		return Key{Name: "alt-enter"}, 1 + size, false
	}
	if r == 0x7f {
		return Key{Name: "alt-bspace"}, 1 + size, false
	}
	return Key{Name: "alt-" + string(r)}, 1 + size, false
}

func decodeCSI(buf []byte) (Key, int, bool) {
	// CSI params final, where final is in @..~
	end := -1
	for i := 2; i < len(buf); i++ {
		if buf[i] >= '@' && buf[i] <= '~' {
			end = i
			break
		}
	}
	if end < 0 {
		if len(buf) > 24 {
			return Key{Name: "esc"}, 1, false // runaway, resynchronise
		}
		return Key{}, 0, true
	}
	seq := string(buf[2 : end+1])
	n := end + 1
	if name, ok := csiKeys[seq]; ok {
		return Key{Name: name}, n, false
	}
	// Modified keys such as "1;5A" (ctrl-up): fall back to the unmodified key
	// rather than dropping the press entirely.
	if i := strings.IndexByte(seq, ';'); i >= 0 {
		if name, ok := csiKeys[seq[:i]+seq[len(seq)-1:]]; ok {
			return Key{Name: name}, n, false
		}
		if name, ok := csiKeys[seq[len(seq)-1:]]; ok {
			return Key{Name: name}, n, false
		}
	}
	return Key{Name: "unknown"}, n, false
}
