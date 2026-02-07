package validator

import (
	"strconv"
	"strings"
)

// Pointer is an RFC 6901 JSON Pointer held as unescaped tokens. An empty
// Pointer is the document root.
type Pointer []string

// push returns a copy with token appended. It never shares the backing array,
// so sibling branches can't overwrite each other's paths.
func (p Pointer) push(token string) Pointer {
	out := make(Pointer, len(p)+1)
	copy(out, p)
	out[len(p)] = token
	return out
}

func (p Pointer) pushIndex(i int) Pointer {
	return p.push(strconv.Itoa(i))
}

// String returns the escaped pointer; the root is "".
func (p Pointer) String() string {
	var b strings.Builder
	for _, tok := range p {
		b.WriteByte('/')
		b.WriteString(escapeToken(tok))
	}
	return b.String()
}

// Human is like String but shows the root as "(root)".
func (p Pointer) Human() string {
	if len(p) == 0 {
		return "(root)"
	}
	return p.String()
}

// escapeToken escapes ~ before / so the ~ in a new ~1 isn't escaped again.
func escapeToken(tok string) string {
	tok = strings.ReplaceAll(tok, "~", "~0")
	tok = strings.ReplaceAll(tok, "/", "~1")
	return tok
}

// parsePointer splits a pointer string into unescaped tokens. "" is the root.
func parsePointer(s string) ([]string, bool) {
	if s == "" {
		return nil, true
	}
	if !strings.HasPrefix(s, "/") {
		return nil, false
	}
	parts := strings.Split(s[1:], "/")
	out := make([]string, len(parts))
	for i, part := range parts {
		out[i] = unescapeToken(part)
	}
	return out, true
}

// unescapeToken decodes ~1 before ~0, otherwise "~01" would wrongly become "/".
func unescapeToken(tok string) string {
	tok = strings.ReplaceAll(tok, "~1", "/")
	tok = strings.ReplaceAll(tok, "~0", "~")
	return tok
}
