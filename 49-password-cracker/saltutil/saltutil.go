// Package saltutil prepends or appends a fixed salt before hashing. Per-user
// salts and KDF formats are out of scope.
package saltutil

import "fmt"

type Mode string

const (
	Prefix Mode = "prefix" // H(salt + pw)
	Suffix Mode = "suffix" // H(pw + salt)
)

// ParseMode validates a salt-mode string.
func ParseMode(s string) (Mode, error) {
	switch Mode(s) {
	case Prefix:
		return Prefix, nil
	case Suffix:
		return Suffix, nil
	default:
		return "", fmt.Errorf("invalid salt-mode %q (want prefix|suffix)", s)
	}
}

// Salter joins a salt to candidates. The zero value adds nothing.
type Salter struct {
	salt []byte
	mode Mode
}

// New builds a Salter; an empty salt makes it a no-op.
func New(salt string, mode Mode) Salter {
	return Salter{salt: []byte(salt), mode: mode}
}

// Apply returns the bytes to hash. It allocates each call, so it is safe for
// concurrent use.
func (s Salter) Apply(candidate string) []byte {
	if len(s.salt) == 0 {
		return []byte(candidate)
	}
	cb := []byte(candidate)
	out := make([]byte, 0, len(s.salt)+len(cb))
	if s.mode == Suffix {
		return append(append(out, cb...), s.salt...)
	}
	return append(append(out, s.salt...), cb...)
}
