package query

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// tokenType enumerates the kinds of lexical tokens in the filter language.
type tokenType int

const (
	tEOF tokenType = iota
	tDot
	tLBracket // [
	tRBracket // ]
	tPipe     // |
	tQuestion // ?
	tIdent    // foo
	tString   // "foo"
	tInt      // 123 / -1
)

func (t tokenType) String() string {
	switch t {
	case tEOF:
		return "end of input"
	case tDot:
		return "'.'"
	case tLBracket:
		return "'['"
	case tRBracket:
		return "']'"
	case tPipe:
		return "'|'"
	case tQuestion:
		return "'?'"
	case tIdent:
		return "identifier"
	case tString:
		return "string"
	case tInt:
		return "integer"
	default:
		return "unknown"
	}
}

// token is a single lexical unit with its source position (byte offset, 1-based column).
type token struct {
	typ tokenType
	val string // literal value for ident/string/int
	pos int    // 1-based column for error reporting
}

// lexError describes a lexical error and its column.
type lexError struct {
	pos int
	msg string
}

func (e *lexError) Error() string {
	return fmt.Sprintf("filter syntax error at column %d: %s", e.pos, e.msg)
}

// lex tokenizes the whole filter string, returning the token slice (ending in tEOF).
func lex(src string) ([]token, error) {
	var toks []token
	i := 0
	for i < len(src) {
		c := src[i]
		col := i + 1
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '.':
			toks = append(toks, token{tDot, "", col})
			i++
		case c == '[':
			toks = append(toks, token{tLBracket, "", col})
			i++
		case c == ']':
			toks = append(toks, token{tRBracket, "", col})
			i++
		case c == '|':
			toks = append(toks, token{tPipe, "", col})
			i++
		case c == '?':
			toks = append(toks, token{tQuestion, "", col})
			i++
		case c == '"':
			s, next, err := lexString(src, i)
			if err != nil {
				return nil, err
			}
			toks = append(toks, token{tString, s, col})
			i = next
		case c == '-' || (c >= '0' && c <= '9'):
			// integer, possibly negative
			n, next, err := lexInt(src, i)
			if err != nil {
				return nil, err
			}
			toks = append(toks, token{tInt, n, col})
			i = next
		case isIdentStart(rune(c)):
			id, next := lexIdent(src, i)
			toks = append(toks, token{tIdent, id, col})
			i = next
		default:
			r, _ := utf8.DecodeRuneInString(src[i:])
			return nil, &lexError{col, fmt.Sprintf("unexpected character %q", r)}
		}
	}
	toks = append(toks, token{tEOF, "", len(src) + 1})
	return toks, nil
}

func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isIdentPart(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func lexIdent(src string, start int) (string, int) {
	i := start
	for i < len(src) {
		r, sz := utf8.DecodeRuneInString(src[i:])
		if !isIdentPart(r) {
			break
		}
		i += sz
	}
	return src[start:i], i
}

func lexInt(src string, start int) (string, int, error) {
	i := start
	if src[i] == '-' {
		i++
		if i >= len(src) || src[i] < '0' || src[i] > '9' {
			return "", 0, &lexError{start + 1, "expected digits after '-'"}
		}
	}
	for i < len(src) && src[i] >= '0' && src[i] <= '9' {
		i++
	}
	return src[start:i], i, nil
}

// lexString scans a double-quoted string starting at src[start] == '"'.
func lexString(src string, start int) (string, int, error) {
	var b strings.Builder
	i := start + 1
	for i < len(src) {
		c := src[i]
		if c == '"' {
			return b.String(), i + 1, nil
		}
		if c == '\\' {
			i++
			if i >= len(src) {
				break
			}
			switch src[i] {
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			case '/':
				b.WriteByte('/')
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			default:
				return "", 0, &lexError{i + 1, fmt.Sprintf("invalid escape \\%c", src[i])}
			}
			i++
			continue
		}
		b.WriteByte(c)
		i++
	}
	return "", 0, &lexError{start + 1, "unterminated string literal"}
}
