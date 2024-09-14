// Package term handles the real terminal: raw mode, decoding keypresses and
// drawing frames. Key decoding works on any io.Reader, so it can be tested.
package term

import (
	"bufio"
	"io"
	"unicode/utf8"
)

// KeyType says which key was pressed. KeyRune covers all ordinary text.
type KeyType int

const (
	KeyRune KeyType = iota
	KeyArrowUp
	KeyArrowDown
	KeyArrowLeft
	KeyArrowRight
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown
	KeyDelete
	KeyBackspace
	KeyEnter
	KeyEsc
	KeyCtrl // Ctrl+letter; Key.Rune is the lowercase letter
)

// Key is one keypress.
type Key struct {
	Type KeyType
	Rune rune // for KeyRune and KeyCtrl
}

// RuneKey returns the Key for typing r.
func RuneKey(r rune) Key { return Key{Type: KeyRune, Rune: r} }

// CtrlKey returns the Key for Ctrl+r.
func CtrlKey(r rune) Key { return Key{Type: KeyCtrl, Rune: r} }

// KeyReader decodes keypresses from raw terminal input, including the VT and
// xterm escape sequences for arrows and the like.
type KeyReader struct {
	r *bufio.Reader
}

// NewKeyReader returns a KeyReader reading from r.
func NewKeyReader(r io.Reader) *KeyReader {
	return &KeyReader{r: bufio.NewReader(r)}
}

// ReadKey blocks until a whole key has been read.
func (k *KeyReader) ReadKey() (Key, error) {
	b, err := k.r.ReadByte()
	if err != nil {
		return Key{}, err
	}

	switch {
	case b == '\r' || b == '\n':
		return Key{Type: KeyEnter}, nil
	case b == 0x7f || b == 0x08: // DEL or BS
		return Key{Type: KeyBackspace}, nil
	case b == 0x1b:
		return k.readEscape()
	case b < 0x20: // Ctrl+letter
		return Key{Type: KeyCtrl, Rune: rune(b - 1 + 'a')}, nil
	default:
		return k.readRune(b)
	}
}

// readRune reads the rest of a UTF-8 character whose first byte is b.
func (k *KeyReader) readRune(b byte) (Key, error) {
	if b < 0x80 {
		return Key{Type: KeyRune, Rune: rune(b)}, nil
	}
	var n int
	switch {
	case b&0xe0 == 0xc0:
		n = 1
	case b&0xf0 == 0xe0:
		n = 2
	case b&0xf8 == 0xf0:
		n = 3
	default:
		// Not a valid lead byte; pass it through as is.
		return Key{Type: KeyRune, Rune: rune(b)}, nil
	}
	buf := []byte{b}
	for i := 0; i < n; i++ {
		c, err := k.r.ReadByte()
		if err != nil {
			break
		}
		buf = append(buf, c)
	}
	r, _ := utf8.DecodeRune(buf)
	return Key{Type: KeyRune, Rune: r}, nil
}

// readEscape decodes what follows an ESC byte. If nothing follows straight
// away, the user pressed Escape itself.
func (k *KeyReader) readEscape() (Key, error) {
	if k.r.Buffered() == 0 {
		return Key{Type: KeyEsc}, nil
	}
	b1, err := k.r.ReadByte()
	if err != nil {
		return Key{Type: KeyEsc}, nil
	}
	if b1 != '[' && b1 != 'O' {
		// Escape followed by an ordinary key; leave that key for the next read.
		_ = k.r.UnreadByte()
		return Key{Type: KeyEsc}, nil
	}
	b2, err := k.r.ReadByte()
	if err != nil {
		return Key{Type: KeyEsc}, nil
	}
	if b2 >= '0' && b2 <= '9' {
		// ESC [ <digit> ~
		num := b2 - '0'
		b3, err := k.r.ReadByte()
		if err != nil {
			return Key{Type: KeyEsc}, nil
		}
		if b3 == '~' {
			switch num {
			case 1, 7:
				return Key{Type: KeyHome}, nil
			case 4, 8:
				return Key{Type: KeyEnd}, nil
			case 3:
				return Key{Type: KeyDelete}, nil
			case 5:
				return Key{Type: KeyPageUp}, nil
			case 6:
				return Key{Type: KeyPageDown}, nil
			}
		}
		return Key{Type: KeyEsc}, nil
	}
	switch b2 {
	case 'A':
		return Key{Type: KeyArrowUp}, nil
	case 'B':
		return Key{Type: KeyArrowDown}, nil
	case 'C':
		return Key{Type: KeyArrowRight}, nil
	case 'D':
		return Key{Type: KeyArrowLeft}, nil
	case 'H':
		return Key{Type: KeyHome}, nil
	case 'F':
		return Key{Type: KeyEnd}, nil
	}
	return Key{Type: KeyEsc}, nil
}
