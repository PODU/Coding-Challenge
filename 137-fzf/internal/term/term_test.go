package term

import "testing"

func decodeAll(t *testing.T, input string) []Key {
	t.Helper()
	var keys []Key
	buf := []byte(input)
	for len(buf) > 0 {
		key, n, more := decodeOne(buf)
		if more {
			// The real decoder would wait for more bytes; in a test the input
			// is complete, so an incomplete sequence is a lone Escape.
			keys = append(keys, Key{Name: "esc"})
			buf = buf[1:]
			continue
		}
		keys = append(keys, key)
		buf = buf[n:]
	}
	return keys
}

func names(keys []Key) []string {
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = k.Name
	}
	return out
}

func assertKeys(t *testing.T, input string, want ...string) {
	t.Helper()
	got := names(decodeAll(t, input))
	if len(got) != len(want) {
		t.Fatalf("%q: got %q, want %q", input, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%q: got %q, want %q", input, got, want)
		}
	}
}

func TestDecodePrintable(t *testing.T) {
	assertKeys(t, "abc", "a", "b", "c")
	assertKeys(t, "a b", "a", "space", "b")
	keys := decodeAll(t, "x")
	if !keys[0].Printable() || keys[0].Rune != 'x' {
		t.Errorf("%+v", keys[0])
	}
}

func TestDecodeControlKeys(t *testing.T) {
	assertKeys(t, "\x01\x03\x17\x15", "ctrl-a", "ctrl-c", "ctrl-w", "ctrl-u")
	assertKeys(t, "\r", "enter")
	assertKeys(t, "\t", "tab")
	assertKeys(t, "\x7f", "bspace")
	assertKeys(t, "\x08", "bspace")
	// Ctrl-J and Ctrl-K are ordinary control characters, not Enter.
	assertKeys(t, "\x0a\x0b", "ctrl-j", "ctrl-k")
}

func TestDecodeArrows(t *testing.T) {
	assertKeys(t, "\x1b[A\x1b[B\x1b[C\x1b[D", "up", "down", "right", "left")
	assertKeys(t, "\x1b[Z", "shift-tab")
	assertKeys(t, "\x1b[5~\x1b[6~", "pgup", "pgdn")
	assertKeys(t, "\x1b[3~", "delete")
	assertKeys(t, "\x1b[H\x1b[F", "home", "end")
}

func TestDecodeFunctionKeys(t *testing.T) {
	assertKeys(t, "\x1bOP\x1bOQ", "f1", "f2")
	assertKeys(t, "\x1b[15~\x1b[24~", "f5", "f12")
}

// A modified arrow key should still move the cursor rather than vanish.
func TestDecodeModifiedArrowFallsBack(t *testing.T) {
	assertKeys(t, "\x1b[1;5A", "up")
}

func TestDecodeAlt(t *testing.T) {
	assertKeys(t, "\x1ba", "alt-a")
	assertKeys(t, "\x1b\r", "alt-enter")
	assertKeys(t, "\x1b\x1b", "esc")
}

func TestDecodeUnicode(t *testing.T) {
	keys := decodeAll(t, "héllo")
	if len(keys) != 5 || keys[1].Rune != 'é' {
		t.Errorf("got %q", names(keys))
	}
}

func TestIncompleteSequenceWantsMoreBytes(t *testing.T) {
	for _, partial := range []string{"\x1b", "\x1b[", "\x1b[1;5"} {
		if _, _, more := decodeOne([]byte(partial)); !more {
			t.Errorf("%q should ask for more bytes", partial)
		}
	}
}

func TestPasteOfMixedInput(t *testing.T) {
	assertKeys(t, "ab\x1b[Ac\r", "a", "b", "up", "c", "enter")
}
