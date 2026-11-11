package term

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

type Key uint8

const (
	KeyRune Key = iota
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyEnter
	KeyTab
	KeyBacktab
	KeyBackspace
	KeyEsc
	KeyHome
	KeyEnd
	KeyDelete
	KeyPgUp
	KeyPgDn
	KeyF5
	KeyCtrlC
	KeyMouse
	KeyUnknown
)

type Event struct {
	Key  Key
	Rune rune
	// Mouse: zero-based cell, button (0 left, 1 middle, 2 right, 64/65
	// wheel), and whether it is a press (true) or a release.
	X, Y   int
	Button int
	Press  bool
}

// Parse splits one read from the terminal into events. Escape sequences
// are assumed to arrive whole, which holds for terminals in practice.
func Parse(b []byte) []Event {
	var evs []Event
	for len(b) > 0 {
		ev, n := parseOne(b)
		evs = append(evs, ev)
		b = b[n:]
	}
	return evs
}

func parseOne(b []byte) (Event, int) {
	switch c := b[0]; {
	case c == 0x1b:
		return parseEscape(b)
	case c == '\r' || c == '\n':
		return Event{Key: KeyEnter}, 1
	case c == '\t':
		return Event{Key: KeyTab}, 1
	case c == 0x7f || c == 0x08:
		return Event{Key: KeyBackspace}, 1
	case c == 0x03:
		return Event{Key: KeyCtrlC}, 1
	case c < 0x20:
		return Event{Key: KeyUnknown}, 1
	}
	r, n := utf8.DecodeRune(b)
	return Event{Key: KeyRune, Rune: r}, n
}

func parseEscape(b []byte) (Event, int) {
	if len(b) == 1 {
		return Event{Key: KeyEsc}, 1
	}
	switch b[1] {
	case '[':
		end := 2
		for end < len(b) && !(b[end] >= 0x40 && b[end] <= 0x7e) {
			end++
		}
		if end >= len(b) {
			return Event{Key: KeyUnknown}, len(b)
		}
		params, final := string(b[2:end]), b[end]
		n := end + 1
		if strings.HasPrefix(params, "<") && (final == 'M' || final == 'm') {
			return parseSGRMouse(params[1:], final == 'M'), n
		}
		switch final {
		case 'A':
			return Event{Key: KeyUp}, n
		case 'B':
			return Event{Key: KeyDown}, n
		case 'C':
			return Event{Key: KeyRight}, n
		case 'D':
			return Event{Key: KeyLeft}, n
		case 'H':
			return Event{Key: KeyHome}, n
		case 'F':
			return Event{Key: KeyEnd}, n
		case 'Z':
			return Event{Key: KeyBacktab}, n
		case '~':
			switch params {
			case "1", "7":
				return Event{Key: KeyHome}, n
			case "4", "8":
				return Event{Key: KeyEnd}, n
			case "3":
				return Event{Key: KeyDelete}, n
			case "5":
				return Event{Key: KeyPgUp}, n
			case "6":
				return Event{Key: KeyPgDn}, n
			case "15":
				return Event{Key: KeyF5}, n
			}
		}
		return Event{Key: KeyUnknown}, n
	case 'O':
		if len(b) < 3 {
			return Event{Key: KeyUnknown}, len(b)
		}
		switch b[2] {
		case 'A':
			return Event{Key: KeyUp}, 3
		case 'B':
			return Event{Key: KeyDown}, 3
		case 'C':
			return Event{Key: KeyRight}, 3
		case 'D':
			return Event{Key: KeyLeft}, 3
		case 'H':
			return Event{Key: KeyHome}, 3
		case 'F':
			return Event{Key: KeyEnd}, 3
		}
		return Event{Key: KeyUnknown}, 3
	}
	return Event{Key: KeyEsc}, 1 // Esc followed by an ordinary key
}

func parseSGRMouse(p string, press bool) Event {
	parts := strings.Split(p, ";")
	if len(parts) != 3 {
		return Event{Key: KeyUnknown}
	}
	b, _ := strconv.Atoi(parts[0])
	x, _ := strconv.Atoi(parts[1])
	y, _ := strconv.Atoi(parts[2])
	return Event{Key: KeyMouse, Button: b &^ (4 | 8 | 16 | 32), X: x - 1, Y: y - 1, Press: press}
}
