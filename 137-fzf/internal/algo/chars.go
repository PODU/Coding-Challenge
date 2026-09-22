package algo

import (
	"unicode"
	"unicode/utf8"
)

// Chars is a read-only view of a string as a sequence of characters.
//
// The common case in fzf's workload is a list of ASCII paths, where a string
// is already a random-access array of characters: keeping the string and
// indexing it directly costs no extra memory, which matters when a million
// items are in memory at once. Only strings that actually contain multi-byte
// runes pay for a []rune copy.
type Chars struct {
	s string
	r []rune // nil when s is pure ASCII
}

func NewChars(s string) Chars {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return Chars{s: s, r: []rune(s)}
		}
	}
	return Chars{s: s}
}

func (c Chars) Len() int {
	if c.r != nil {
		return len(c.r)
	}
	return len(c.s)
}

func (c Chars) Get(i int) rune {
	if c.r != nil {
		return c.r[i]
	}
	return rune(c.s[i])
}

func (c Chars) String() string { return c.s }

// ToRunes materialises the character slice (used by the renderer, never in the
// matching hot loop).
func (c Chars) ToRunes() []rune {
	if c.r != nil {
		return c.r
	}
	return []rune(c.s)
}

func toLower(r rune) rune {
	if r < utf8.RuneSelf {
		if r >= 'A' && r <= 'Z' {
			return r + 32
		}
		return r
	}
	return unicode.ToLower(r)
}

// ToLowerString is used when a whole pattern needs folding.
func ToLowerString(s string) string {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii {
		b := []byte(s)
		for i, c := range b {
			if c >= 'A' && c <= 'Z' {
				b[i] = c + 32
			}
		}
		return string(b)
	}
	rs := []rune(s)
	for i, r := range rs {
		rs[i] = unicode.ToLower(r)
	}
	return string(rs)
}
