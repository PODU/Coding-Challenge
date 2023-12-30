// Package set parses tr's SET operands: literal characters, escapes
// (\n \t \\ \NNN ...), ranges (a-z), classes ([:alpha:] ...), equivalence
// classes ([=c=]) and, in SET2 only, repeats ([c*n] / [c*]).
package set

import (
	"fmt"
	"strconv"
	"strings"
)

// Set is a parsed SET. It is kept as tokens because [c*] can only be expanded
// once the length of SET1 is known.
type Set struct {
	tokens []token
	raw    string
}

type tokenKind int

const (
	tokChar   tokenKind = iota // lo
	tokRange                   // lo through hi
	tokClass                   // class
	tokRepeat                  // lo repeated count times; count 0 means pad to length
)

type token struct {
	kind  tokenKind
	lo    byte
	hi    byte
	class string
	count int
}

// Parse parses a SET. Backwards ranges and unknown classes are errors.
func Parse(s string) (*Set, error) {
	st := &Set{raw: s}
	i, n := 0, len(s)
	for i < n {
		if name, end, ok := classAt(s, i); ok {
			if _, err := classBytes(name); err != nil {
				return nil, err
			}
			st.tokens = append(st.tokens, token{kind: tokClass, class: name})
			i = end
			continue
		}
		// In the C locale [=c=] is just c.
		if c, end, ok := equivAt(s, i); ok {
			st.tokens = append(st.tokens, token{kind: tokChar, lo: c})
			i = end
			continue
		}
		if rb, rc, end, ok := repeatAt(s, i); ok {
			st.tokens = append(st.tokens, token{kind: tokRepeat, lo: rb, count: rc})
			i = end
			continue
		}
		b, ni := decodeChar(s, i)
		// A '-' at the end, or followed by a bracket form, is a literal '-'.
		if ni+1 < n && s[ni] == '-' && s[ni+1] != '[' {
			hi, next := decodeChar(s, ni+1)
			if hi < b {
				return nil, fmt.Errorf("range-endpoints of '%c-%c' are in reverse collating sequence order", b, hi)
			}
			st.tokens = append(st.tokens, token{kind: tokRange, lo: b, hi: hi})
			i = next
			continue
		}
		st.tokens = append(st.tokens, token{kind: tokChar, lo: b})
		i = ni
	}
	return st, nil
}

// Expand returns the bytes of st in order. For SET2, targetLen is the length
// of SET1 and a [c*] fills whatever is missing. Pass a negative targetLen for
// SET1, where [c*] is not allowed.
func Expand(st *Set, targetLen int) ([]byte, error) {
	fixed := 0
	for _, t := range st.tokens {
		switch t.kind {
		case tokChar:
			fixed++
		case tokRange:
			fixed += int(t.hi) - int(t.lo) + 1
		case tokClass:
			cb, _ := classBytes(t.class)
			fixed += len(cb)
		case tokRepeat:
			if t.count > 0 {
				fixed += t.count
			}
		}
	}
	deficit := 0
	if targetLen > fixed {
		deficit = targetLen - fixed
	}

	var out []byte
	padUsed := false
	for _, t := range st.tokens {
		switch t.kind {
		case tokChar:
			out = append(out, t.lo)
		case tokRange:
			for b := int(t.lo); b <= int(t.hi); b++ {
				out = append(out, byte(b))
			}
		case tokClass:
			cb, _ := classBytes(t.class)
			out = append(out, cb...)
		case tokRepeat:
			if t.count > 0 {
				for k := 0; k < t.count; k++ {
					out = append(out, t.lo)
				}
			} else {
				if targetLen < 0 {
					return nil, fmt.Errorf("the [c*] repeat construct may not appear in string1")
				}
				if !padUsed {
					for k := 0; k < deficit; k++ {
						out = append(out, t.lo)
					}
					padUsed = true
				}
			}
		}
	}
	return out, nil
}

// Set2ClassesOK checks GNU tr's rule that SET2 of a translation may only use
// the [:upper:] and [:lower:] classes.
func (st *Set) Set2ClassesOK() error {
	for _, t := range st.tokens {
		if t.kind == tokClass && t.class != "lower" && t.class != "upper" {
			return fmt.Errorf("the only character classes that may appear in string2 are 'upper' and 'lower'")
		}
	}
	return nil
}

// decodeChar decodes the character or escape at s[i] and returns it with the
// index just past it.
func decodeChar(s string, i int) (byte, int) {
	if s[i] != '\\' {
		return s[i], i + 1
	}
	if i+1 >= len(s) {
		return '\\', i + 1
	}
	c := s[i+1]
	switch c {
	case '\\':
		return '\\', i + 2
	case 'n':
		return '\n', i + 2
	case 't':
		return '\t', i + 2
	case 'r':
		return '\r', i + 2
	case 'f':
		return '\f', i + 2
	case 'v':
		return '\v', i + 2
	case 'a':
		return 0x07, i + 2
	case 'b':
		return 0x08, i + 2
	}
	if c >= '0' && c <= '7' {
		val, j, k := 0, i+1, 0
		for k < 3 && j < len(s) && s[j] >= '0' && s[j] <= '7' {
			val = val*8 + int(s[j]-'0')
			j++
			k++
		}
		return byte(val & 0xFF), j
	}
	// Unknown escape: keep the character. GNU tr does the same, with a warning.
	return c, i + 2
}

func classAt(s string, i int) (name string, end int, ok bool) {
	n := len(s)
	if i+1 >= n || s[i] != '[' || s[i+1] != ':' {
		return "", 0, false
	}
	idx := strings.Index(s[i+2:], ":]")
	if idx < 0 {
		return "", 0, false
	}
	return s[i+2 : i+2+idx], i + 2 + idx + 2, true
}

func equivAt(s string, i int) (b byte, end int, ok bool) {
	n := len(s)
	if i+1 >= n || s[i] != '[' || s[i+1] != '=' {
		return 0, 0, false
	}
	cb, j := decodeChar(s, i+2)
	if j+1 < n && s[j] == '=' && s[j+1] == ']' {
		return cb, j + 2, true
	}
	return 0, 0, false
}

func repeatAt(s string, i int) (b byte, count int, end int, ok bool) {
	n := len(s)
	if i+1 >= n || s[i] != '[' {
		return 0, 0, 0, false
	}
	cb, j := decodeChar(s, i+1)
	if j >= n || s[j] != '*' {
		return 0, 0, 0, false
	}
	k := j + 1
	start := k
	for k < n && s[k] >= '0' && s[k] <= '9' {
		k++
	}
	if k >= n || s[k] != ']' {
		return 0, 0, 0, false
	}
	digits := s[start:k]
	cnt := 0
	if len(digits) > 0 {
		base := 10
		if digits[0] == '0' {
			base = 8
		}
		v, err := strconv.ParseInt(digits, base, 64)
		if err != nil {
			return 0, 0, 0, false
		}
		cnt = int(v)
	}
	return cb, cnt, k + 1, true
}

// classBytes returns the bytes in a character class, in ascending order.
func classBytes(name string) ([]byte, error) {
	var out []byte
	add := func(lo, hi int) {
		for b := lo; b <= hi; b++ {
			out = append(out, byte(b))
		}
	}
	switch name {
	case "alnum":
		add('0', '9')
		add('A', 'Z')
		add('a', 'z')
	case "alpha":
		add('A', 'Z')
		add('a', 'z')
	case "digit":
		add('0', '9')
	case "lower":
		add('a', 'z')
	case "upper":
		add('A', 'Z')
	case "space":
		out = []byte{'\t', '\n', '\v', '\f', '\r', ' '}
	case "blank":
		out = []byte{'\t', ' '}
	case "xdigit":
		add('0', '9')
		add('A', 'F')
		add('a', 'f')
	case "cntrl":
		add(0x00, 0x1F)
		out = append(out, 0x7F)
	case "print":
		add(0x20, 0x7E)
	case "graph":
		add(0x21, 0x7E)
	case "punct":
		for b := 0x21; b <= 0x7E; b++ {
			c := byte(b)
			if !(c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z') {
				out = append(out, c)
			}
		}
	default:
		return nil, fmt.Errorf("invalid character class %q", name)
	}
	return out, nil
}
