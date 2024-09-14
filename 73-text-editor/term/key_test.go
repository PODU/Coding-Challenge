package term

import (
	"strings"
	"testing"
)

func decodeAll(s string) []Key {
	kr := NewKeyReader(strings.NewReader(s))
	var keys []Key
	for {
		k, err := kr.ReadKey()
		if err != nil {
			break
		}
		keys = append(keys, k)
	}
	return keys
}

func TestDecodeRunes(t *testing.T) {
	keys := decodeAll("ab")
	if len(keys) != 2 || keys[0].Type != KeyRune || keys[0].Rune != 'a' {
		t.Fatalf("got %+v", keys)
	}
}

func TestDecodeArrows(t *testing.T) {
	keys := decodeAll("\x1b[A\x1b[B\x1b[C\x1b[D")
	want := []KeyType{KeyArrowUp, KeyArrowDown, KeyArrowRight, KeyArrowLeft}
	if len(keys) != 4 {
		t.Fatalf("got %d keys, want 4: %+v", len(keys), keys)
	}
	for i, w := range want {
		if keys[i].Type != w {
			t.Errorf("key %d = %v, want %v", i, keys[i].Type, w)
		}
	}
}

func TestDecodeSpecial(t *testing.T) {
	cases := map[string]KeyType{
		"\x1b[3~": KeyDelete,
		"\x1b[5~": KeyPageUp,
		"\x1b[6~": KeyPageDown,
		"\x1b[1~": KeyHome,
		"\x1b[4~": KeyEnd,
		"\x1b[H":  KeyHome,
		"\x1b[F":  KeyEnd,
	}
	for in, want := range cases {
		keys := decodeAll(in)
		if len(keys) != 1 || keys[0].Type != want {
			t.Errorf("decode %q = %+v, want type %v", in, keys, want)
		}
	}
}

func TestDecodeBareEscape(t *testing.T) {
	keys := decodeAll("\x1b")
	if len(keys) != 1 || keys[0].Type != KeyEsc {
		t.Fatalf("bare ESC = %+v, want Esc", keys)
	}
}

func TestDecodeEnterBackspaceCtrl(t *testing.T) {
	keys := decodeAll("\r\x7f\x06") // Enter, Backspace, Ctrl-F
	if keys[0].Type != KeyEnter {
		t.Errorf("0 = %v, want Enter", keys[0].Type)
	}
	if keys[1].Type != KeyBackspace {
		t.Errorf("1 = %v, want Backspace", keys[1].Type)
	}
	if keys[2].Type != KeyCtrl || keys[2].Rune != 'f' {
		t.Errorf("2 = %+v, want Ctrl-f", keys[2])
	}
}

func TestDecodeUTF8(t *testing.T) {
	keys := decodeAll("é")
	if len(keys) != 1 || keys[0].Type != KeyRune || keys[0].Rune != 'é' {
		t.Fatalf("utf8 decode = %+v", keys)
	}
}
