// Package scanner is the lexical layer: it splits input into indentation-aware
// lines and decodes single- and double-quoted scalars. Package parser builds
// the structure.
package scanner

import (
	"fmt"
	"strings"
)

// Line is one input line with its indentation measured. Indent counts leading
// spaces and Text is the rest. HasTabIndent flags a tab in the indentation,
// which the parser rejects structurally but allows inside block scalars.
type Line struct {
	Num          int // 1-based
	Indent       int
	Text         string
	HasTabIndent bool
}

// Blank reports whether the line has no content (only indentation).
func (l Line) Blank() bool { return l.Text == "" }

// Comment reports whether the line's content is a full-line comment.
func (l Line) Comment() bool { return strings.HasPrefix(l.Text, "#") }

// Raw reconstructs the original line (indentation + text).
func (l Line) Raw() string { return strings.Repeat(" ", l.Indent) + l.Text }

// SplitLines normalises line endings and splits src into measured lines.
func SplitLines(src []byte) []Line {
	raw := strings.Split(normalizeNewlines(string(src)), "\n")
	// Drop the empty element a trailing newline leaves, so EOF doesn't look
	// like an extra blank line.
	if len(raw) > 0 && raw[len(raw)-1] == "" {
		raw = raw[:len(raw)-1]
	}
	lines := make([]Line, 0, len(raw))
	for i, r := range raw {
		indent := 0
		for indent < len(r) && r[indent] == ' ' {
			indent++
		}
		lines = append(lines, Line{
			Num:          i + 1,
			Indent:       indent,
			Text:         r[indent:],
			HasTabIndent: indent < len(r) && r[indent] == '\t',
		})
	}
	return lines
}

func normalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

// ReadSingleQuoted reads a single-quoted scalar; s[i] must be the opening
// quote. The only escape is a doubled quote, and a newline folds to a space.
// It returns the value and the index past the closing quote.
func ReadSingleQuoted(s string, i int) (string, int, error) {
	if i >= len(s) || s[i] != '\'' {
		return "", i, fmt.Errorf("expected single quote")
	}
	i++
	var b strings.Builder
	for i < len(s) {
		c := s[i]
		switch c {
		case '\'':
			if i+1 < len(s) && s[i+1] == '\'' {
				b.WriteByte('\'')
				i += 2
				continue
			}
			return b.String(), i + 1, nil
		case '\n':
			b.WriteByte(' ')
			i++
		default:
			b.WriteByte(c)
			i++
		}
	}
	return "", i, fmt.Errorf("unterminated single-quoted string")
}

// ReadDoubleQuoted reads a double-quoted scalar; s[i] must be the opening
// quote. It handles C-style and \x/\u/\U escapes, folds a newline to a space,
// and returns the value and the index past the closing quote.
func ReadDoubleQuoted(s string, i int) (string, int, error) {
	if i >= len(s) || s[i] != '"' {
		return "", i, fmt.Errorf("expected double quote")
	}
	i++
	var b strings.Builder
	for i < len(s) {
		c := s[i]
		switch c {
		case '"':
			return b.String(), i + 1, nil
		case '\n':
			b.WriteByte(' ')
			i++
		case '\\':
			r, ni, err := decodeEscape(s, i)
			if err != nil {
				return "", i, err
			}
			b.WriteString(r)
			i = ni
		default:
			b.WriteByte(c)
			i++
		}
	}
	return "", i, fmt.Errorf("unterminated double-quoted string")
}

// decodeEscape decodes the escape at s[i] (a backslash) and returns its text
// and the index past it.
func decodeEscape(s string, i int) (string, int, error) {
	if i+1 >= len(s) {
		return "", i, fmt.Errorf("dangling escape at end of string")
	}
	c := s[i+1]
	switch c {
	case 'n':
		return "\n", i + 2, nil
	case 't':
		return "\t", i + 2, nil
	case 'r':
		return "\r", i + 2, nil
	case '"':
		return "\"", i + 2, nil
	case '\\':
		return "\\", i + 2, nil
	case '/':
		return "/", i + 2, nil
	case '0':
		return "\x00", i + 2, nil
	case 'a':
		return "\a", i + 2, nil
	case 'b':
		return "\b", i + 2, nil
	case 'f':
		return "\f", i + 2, nil
	case 'v':
		return "\v", i + 2, nil
	case 'e':
		return "\x1b", i + 2, nil
	case ' ':
		return " ", i + 2, nil
	case 'x':
		return decodeHex(s, i+2, 2)
	case 'u':
		return decodeHex(s, i+2, 4)
	case 'U':
		return decodeHex(s, i+2, 8)
	default:
		return "", i, fmt.Errorf("invalid escape sequence \\%c", c)
	}
}

func decodeHex(s string, i, n int) (string, int, error) {
	if i+n > len(s) {
		return "", i, fmt.Errorf("incomplete unicode escape")
	}
	var cp rune
	for k := 0; k < n; k++ {
		d := s[i+k]
		var v rune
		switch {
		case d >= '0' && d <= '9':
			v = rune(d - '0')
		case d >= 'a' && d <= 'f':
			v = rune(d-'a') + 10
		case d >= 'A' && d <= 'F':
			v = rune(d-'A') + 10
		default:
			return "", i, fmt.Errorf("invalid hex digit %q in unicode escape", string(d))
		}
		cp = cp<<4 | v
	}
	// \xHH is a Latin-1 code point; \u and \U are full code points.
	return string(cp), i + n, nil
}
