package jq

import (
	"fmt"
	"strconv"
	"strings"
)

type tokKind int

const (
	tEOF tokKind = iota
	tDot
	tDotDot
	tField    // .name   (val = name)
	tLBracket // [
	tRBracket // ]
	tLBrace   // {
	tRBrace   // }
	tLParen   // (
	tRParen   // )
	tPipe     // |
	tComma    // ,
	tColon    // :
	tSemicolon
	tQuestion // ?
	tNumber   // val = number text
	tString   // parts = string segments
	tIdent    // val = identifier / keyword
	tFormat   // @name
	tOp       // val = operator text (== != < <= > >= + - * / %  // and or)
)

// strSeg is one segment of a (possibly interpolated) string literal: either a
// literal text run (IsExpr false) or an interpolation \(...) (IsExpr true, the
// raw sub-expression source in Expr).
type strSeg struct {
	IsExpr bool
	Text   string
	Expr   string
}

type token struct {
	kind  tokKind
	val   string
	num   float64
	parts []strSeg
	pos   int
}

type lexer struct {
	src  string
	pos  int
	toks []token
}

func lex(src string) ([]token, error) {
	l := &lexer{src: src}
	if err := l.run(); err != nil {
		return nil, err
	}
	return l.toks, nil
}

func (l *lexer) emit(t token) { l.toks = append(l.toks, t) }

func (l *lexer) run() error {
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		start := l.pos
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			l.pos++
		case c == '#':
			for l.pos < len(l.src) && l.src[l.pos] != '\n' {
				l.pos++
			}
		case c == '.':
			if err := l.lexDot(); err != nil {
				return err
			}
		case c == '[':
			l.pos++
			l.emit(token{kind: tLBracket, pos: start})
		case c == ']':
			l.pos++
			l.emit(token{kind: tRBracket, pos: start})
		case c == '{':
			l.pos++
			l.emit(token{kind: tLBrace, pos: start})
		case c == '}':
			l.pos++
			l.emit(token{kind: tRBrace, pos: start})
		case c == '(':
			l.pos++
			l.emit(token{kind: tLParen, pos: start})
		case c == ')':
			l.pos++
			l.emit(token{kind: tRParen, pos: start})
		case c == '|':
			l.pos++
			l.emit(token{kind: tPipe, pos: start})
		case c == ',':
			l.pos++
			l.emit(token{kind: tComma, pos: start})
		case c == ':':
			l.pos++
			l.emit(token{kind: tColon, pos: start})
		case c == ';':
			l.pos++
			l.emit(token{kind: tSemicolon, pos: start})
		case c == '?':
			l.pos++
			l.emit(token{kind: tQuestion, pos: start})
		case c == '"':
			if err := l.lexString(); err != nil {
				return err
			}
		case c == '@':
			l.lexFormat()
		case c >= '0' && c <= '9':
			if err := l.lexNumber(); err != nil {
				return err
			}
		case isIdentStart(c):
			l.lexIdent()
		case strings.ContainsRune("=!<>+-*/%", rune(c)):
			if err := l.lexOp(); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unexpected character %q at offset %d", string(c), l.pos)
		}
	}
	l.emit(token{kind: tEOF, pos: l.pos})
	return nil
}

func (l *lexer) lexDot() error {
	start := l.pos
	l.pos++ // consume '.'
	if l.pos < len(l.src) && l.src[l.pos] == '.' {
		l.pos++
		l.emit(token{kind: tDotDot, pos: start})
		return nil
	}
	if l.pos < len(l.src) && isIdentStart(l.src[l.pos]) {
		name := l.scanIdent()
		l.emit(token{kind: tField, val: name, pos: start})
		return nil
	}
	l.emit(token{kind: tDot, pos: start})
	return nil
}

func (l *lexer) lexFormat() {
	start := l.pos
	l.pos++ // '@'
	name := l.scanIdent()
	l.emit(token{kind: tFormat, val: name, pos: start})
}

func (l *lexer) lexIdent() {
	start := l.pos
	name := l.scanIdent()
	l.emit(token{kind: tIdent, val: name, pos: start})
}

func (l *lexer) scanIdent() string {
	s := l.pos
	for l.pos < len(l.src) && isIdentPart(l.src[l.pos]) {
		l.pos++
	}
	return l.src[s:l.pos]
}

func (l *lexer) lexNumber() error {
	start := l.pos
	for l.pos < len(l.src) && (isDigit(l.src[l.pos]) || l.src[l.pos] == '.') {
		l.pos++
	}
	if l.pos < len(l.src) && (l.src[l.pos] == 'e' || l.src[l.pos] == 'E') {
		l.pos++
		if l.pos < len(l.src) && (l.src[l.pos] == '+' || l.src[l.pos] == '-') {
			l.pos++
		}
		for l.pos < len(l.src) && isDigit(l.src[l.pos]) {
			l.pos++
		}
	}
	text := l.src[start:l.pos]
	f, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return fmt.Errorf("invalid number %q at offset %d", text, start)
	}
	l.emit(token{kind: tNumber, val: text, num: f, pos: start})
	return nil
}

func (l *lexer) lexOp() error {
	start := l.pos
	c := l.src[l.pos]
	two := ""
	if l.pos+1 < len(l.src) {
		two = l.src[l.pos : l.pos+2]
	}
	switch {
	case two == "==" || two == "!=" || two == "<=" || two == ">=" || two == "//":
		l.pos += 2
		l.emit(token{kind: tOp, val: two, pos: start})
	case c == '!':
		return fmt.Errorf("unexpected character %q at offset %d", "!", start)
	default:
		l.pos++
		l.emit(token{kind: tOp, val: string(c), pos: start})
	}
	return nil
}

func (l *lexer) lexString() error {
	start := l.pos
	l.pos++ // consume opening quote
	var segs []strSeg
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			segs = append(segs, strSeg{Text: cur.String()})
			cur.Reset()
		}
	}
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch c {
		case '"':
			l.pos++
			flush()
			if segs == nil {
				segs = []strSeg{{Text: ""}}
			}
			l.emit(token{kind: tString, parts: segs, pos: start})
			return nil
		case '\\':
			if l.pos+1 >= len(l.src) {
				return fmt.Errorf("unterminated escape in string at offset %d", l.pos)
			}
			n := l.src[l.pos+1]
			if n == '(' {
				// interpolation \( expr )
				flush()
				exprSrc, np, err := l.scanInterp(l.pos + 2)
				if err != nil {
					return err
				}
				segs = append(segs, strSeg{IsExpr: true, Expr: exprSrc})
				l.pos = np
				continue
			}
			r, np, err := decodeEscape(l.src, l.pos)
			if err != nil {
				return err
			}
			cur.WriteRune(r)
			l.pos = np
		default:
			cur.WriteByte(c)
			l.pos++
		}
	}
	return fmt.Errorf("unterminated string literal at offset %d", start)
}

// scanInterp scans a balanced \( ... ) interpolation starting just after the
// '(' and returns the raw inner source plus the index just past the ')'.
func (l *lexer) scanInterp(from int) (string, int, error) {
	depth := 1
	i := from
	for i < len(l.src) {
		c := l.src[i]
		switch c {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return l.src[from:i], i + 1, nil
			}
		case '"':
			// skip nested string literals so their parens and quotes don't count
			j, err := skipString(l.src, i)
			if err != nil {
				return "", 0, err
			}
			i = j
			continue
		}
		i++
	}
	return "", 0, fmt.Errorf("unterminated interpolation at offset %d", from)
}

// skipString returns the index just past the string literal beginning at i.
func skipString(src string, i int) (int, error) {
	i++ // opening quote
	for i < len(src) {
		switch src[i] {
		case '\\':
			i += 2
			continue
		case '"':
			return i + 1, nil
		}
		i++
	}
	return 0, fmt.Errorf("unterminated string literal at offset %d", i)
}

// decodeEscape decodes a single backslash escape at position i (src[i]=='\\')
// and returns the rune plus the next index.
func decodeEscape(src string, i int) (rune, int, error) {
	c := src[i+1]
	switch c {
	case '"':
		return '"', i + 2, nil
	case '\\':
		return '\\', i + 2, nil
	case '/':
		return '/', i + 2, nil
	case 'b':
		return '\b', i + 2, nil
	case 'f':
		return '\f', i + 2, nil
	case 'n':
		return '\n', i + 2, nil
	case 'r':
		return '\r', i + 2, nil
	case 't':
		return '\t', i + 2, nil
	case 'u':
		if i+6 > len(src) {
			return 0, 0, fmt.Errorf("invalid \\u escape at offset %d", i)
		}
		v, err := strconv.ParseUint(src[i+2:i+6], 16, 32)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid \\u escape at offset %d", i)
		}
		r := rune(v)
		if utf16IsHighSurrogate(r) && i+12 <= len(src) && src[i+6] == '\\' && src[i+7] == 'u' {
			v2, err := strconv.ParseUint(src[i+8:i+12], 16, 32)
			if err == nil {
				r2 := rune(v2)
				if utf16IsLowSurrogate(r2) {
					return utf16Decode(r, r2), i + 12, nil
				}
			}
		}
		return r, i + 6, nil
	default:
		return 0, 0, fmt.Errorf("invalid escape \\%s at offset %d", string(c), i)
	}
}

func utf16IsHighSurrogate(r rune) bool { return r >= 0xd800 && r <= 0xdbff }
func utf16IsLowSurrogate(r rune) bool  { return r >= 0xdc00 && r <= 0xdfff }
func utf16Decode(r1, r2 rune) rune {
	return (r1-0xd800)<<10 + (r2 - 0xdc00) + 0x10000
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
func isIdentPart(c byte) bool { return isIdentStart(c) || isDigit(c) }
func isDigit(c byte) bool     { return c >= '0' && c <= '9' }
