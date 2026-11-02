package main

// EventKind separates plain key bytes from escape sequences, so that pressing
// an arrow key (ESC [ 1 ; 5 A) is not mistaken for the digits 1 and 5.
type EventKind uint8

const (
	EvKey     EventKind = iota // a single byte: letter, digit, control key
	EvSpecial                  // an escape sequence: arrows, function keys, a lone Esc
	EvMouse                    // a mouse report
)

type Event struct {
	Kind EventKind
	Key  byte // for EvKey
}

// parseInput splits one read from the terminal into events. Sequences are
// assumed not to be split across reads, which holds for real terminals since
// each key's sequence is written in one go.
func parseInput(b []byte) []Event {
	var evs []Event
	for i := 0; i < len(b); {
		if b[i] != 0x1b {
			evs = append(evs, Event{Kind: EvKey, Key: b[i]})
			i++
			continue
		}
		n, kind := escapeLen(b[i:])
		evs = append(evs, Event{Kind: kind})
		i += n
	}
	return evs
}

// escapeLen measures the escape sequence at the start of b.
func escapeLen(b []byte) (int, EventKind) {
	if len(b) < 2 {
		return 1, EvSpecial // a lone Esc
	}
	switch b[1] {
	case '[':
		// X10 mouse: ESC [ M followed by three raw bytes.
		if len(b) >= 3 && b[2] == 'M' {
			return min(len(b), 6), EvMouse
		}
		// CSI: parameters then a final byte in 0x40..0x7E.
		for j := 2; j < len(b); j++ {
			if b[j] >= 0x40 && b[j] <= 0x7e {
				if b[2] == '<' && (b[j] == 'M' || b[j] == 'm') {
					return j + 1, EvMouse // SGR mouse: ESC [ < b ; x ; y M
				}
				return j + 1, EvSpecial
			}
		}
		return len(b), EvSpecial
	case 'O':
		return min(len(b), 3), EvSpecial // SS3: F1-F4 and some arrow keys
	default:
		return 2, EvSpecial // Alt+key
	}
}

// action is what the animation loop should do in response to an event.
type action struct {
	quit  bool
	speed int // -1 when unchanged
}

func decide(ev Event, screensaver bool) action {
	none := action{speed: -1}
	if screensaver {
		return action{quit: true, speed: -1}
	}
	if ev.Kind != EvKey {
		return none
	}
	switch k := ev.Key; {
	case k == 'q' || k == 'Q' || k == 0x03: // 0x03 is Ctrl+C in raw mode
		return action{quit: true, speed: -1}
	case k >= '0' && k <= '9':
		return action{speed: int(k - '0')}
	}
	return none
}
