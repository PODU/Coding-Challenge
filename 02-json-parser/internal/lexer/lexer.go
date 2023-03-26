// Package lexer turns RFC 8259 JSON into tokens, tracking line and column for
// error messages.
package lexer

import (
	"strings"
	"unicode/utf8"
)

// Lexer scans a UTF-8 byte buffer into JSON tokens.
type Lexer struct {
	input []byte
	pos   int // byte offset of the next byte
	line  int // 1-based
	col   int // 1-based, counted in runes
}

// New returns a Lexer over input.
func New(input []byte) *Lexer {
	return &Lexer{input: input, line: 1, col: 1}
}

func (l *Lexer) atEnd() bool { return l.pos >= len(l.input) }

// peekByte returns the current byte, or 0 at EOF.
func (l *Lexer) peekByte() byte {
	if l.atEnd() {
		return 0
	}
	return l.input[l.pos]
}

// advanceByte consumes one ASCII byte. Use advanceRune for anything >= 0x80.
func (l *Lexer) advanceByte() byte {
	b := l.input[l.pos]
	l.pos++
	if b == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return b
}

func (l *Lexer) skipWhitespace() {
	for !l.atEnd() {
		switch l.peekByte() {
		case ' ', '\t', '\n', '\r':
			l.advanceByte()
		default:
			return
		}
	}
}

// NextToken returns the next token, or EOF at the end of input. Anything it
// can't tokenize comes back as ILLEGAL with the reason in Literal.
func (l *Lexer) NextToken() Token {
	l.skipWhitespace()

	startLine, startCol := l.line, l.col

	if l.atEnd() {
		return Token{Type: EOF, Line: startLine, Col: startCol}
	}

	b := l.peekByte()
	switch b {
	case '{':
		l.advanceByte()
		return Token{Type: LBRACE, Literal: "{", Line: startLine, Col: startCol}
	case '}':
		l.advanceByte()
		return Token{Type: RBRACE, Literal: "}", Line: startLine, Col: startCol}
	case '[':
		l.advanceByte()
		return Token{Type: LBRACKET, Literal: "[", Line: startLine, Col: startCol}
	case ']':
		l.advanceByte()
		return Token{Type: RBRACKET, Literal: "]", Line: startLine, Col: startCol}
	case ':':
		l.advanceByte()
		return Token{Type: COLON, Literal: ":", Line: startLine, Col: startCol}
	case ',':
		l.advanceByte()
		return Token{Type: COMMA, Literal: ",", Line: startLine, Col: startCol}
	case '"':
		return l.scanString(startLine, startCol)
	case 't':
		return l.scanLiteral("true", TRUE, startLine, startCol)
	case 'f':
		return l.scanLiteral("false", FALSE, startLine, startCol)
	case 'n':
		return l.scanLiteral("null", NULL, startLine, startCol)
	}

	if b == '-' || (b >= '0' && b <= '9') {
		return l.scanNumber(startLine, startCol)
	}

	r, _ := utf8.DecodeRune(l.input[l.pos:])
	if r == utf8.RuneError {
		l.advanceByte()
		return Token{Type: ILLEGAL, Literal: "invalid UTF-8 byte", Line: startLine, Col: startCol}
	}
	l.advanceRune()
	return Token{Type: ILLEGAL, Literal: "unexpected character " + string(r), Line: startLine, Col: startCol}
}

// advanceRune consumes one UTF-8 rune, or a single byte if it isn't valid UTF-8.
func (l *Lexer) advanceRune() rune {
	r, size := utf8.DecodeRune(l.input[l.pos:])
	if r == utf8.RuneError && size == 1 {
		l.pos++
		l.col++
		return r
	}
	l.pos += size
	if r == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return r
}

// scanLiteral matches true, false or null.
func (l *Lexer) scanLiteral(word string, tt TokenType, line, col int) Token {
	if l.pos+len(word) <= len(l.input) && string(l.input[l.pos:l.pos+len(word)]) == word {
		for i := 0; i < len(word); i++ {
			l.advanceByte()
		}
		return Token{Type: tt, Literal: word, Line: line, Col: col}
	}
	// Swallow the whole run of letters so the error names the bad word.
	start := l.pos
	for !l.atEnd() {
		c := l.peekByte()
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			l.advanceByte()
		} else {
			break
		}
	}
	bad := string(l.input[start:l.pos])
	if bad == "" {
		bad = string(l.peekByte())
	}
	return Token{Type: ILLEGAL, Literal: "invalid literal \"" + bad + "\"", Line: line, Col: col}
}

// scanString scans a quoted string and decodes its escapes.
func (l *Lexer) scanString(line, col int) Token {
	l.advanceByte()

	var sb strings.Builder
	for {
		if l.atEnd() {
			return Token{Type: ILLEGAL, Literal: "unterminated string", Line: line, Col: col}
		}
		b := l.peekByte()

		switch {
		case b == '"':
			l.advanceByte()
			return Token{Type: STRING, Literal: sb.String(), Line: line, Col: col}
		case b == '\\':
			l.advanceByte()
			if l.atEnd() {
				return Token{Type: ILLEGAL, Literal: "unterminated string", Line: line, Col: col}
			}
			esc := l.peekByte()
			if c, ok := simpleEscapes[esc]; ok {
				sb.WriteByte(c)
				l.advanceByte()
				continue
			}
			switch esc {
			case 'u':
				eLine, eCol := l.line, l.col
				r, ok := l.scanUnicodeEscape()
				if !ok {
					return Token{Type: ILLEGAL, Literal: "invalid \\u escape sequence", Line: eLine, Col: eCol}
				}
				sb.WriteRune(r)
			default:
				return Token{Type: ILLEGAL, Literal: "invalid escape sequence \\" + string(esc), Line: l.line, Col: l.col}
			}
		case b < 0x20:
			return Token{Type: ILLEGAL, Literal: "unescaped control character in string", Line: l.line, Col: l.col}
		case b < 0x80:
			sb.WriteByte(b)
			l.advanceByte()
		default:
			r, size := utf8.DecodeRune(l.input[l.pos:])
			if r == utf8.RuneError && size == 1 {
				return Token{Type: ILLEGAL, Literal: "invalid UTF-8 in string", Line: l.line, Col: l.col}
			}
			sb.WriteRune(r)
			l.advanceRune()
		}
	}
}

var simpleEscapes = map[byte]byte{
	'"': '"', '\\': '\\', '/': '/',
	'b': '\b', 'f': '\f', 'n': '\n', 'r': '\r', 't': '\t',
}

// scanUnicodeEscape decodes a \uXXXX escape, starting at the 'u', including
// surrogate pairs.
func (l *Lexer) scanUnicodeEscape() (rune, bool) {
	l.advanceByte()
	hi, ok := l.readHex4()
	if !ok {
		return 0, false
	}

	// A high surrogate must be followed by a \u low surrogate.
	if hi >= 0xD800 && hi <= 0xDBFF {
		if l.peekByte() != '\\' {
			return 0, false
		}
		l.advanceByte()
		if l.peekByte() != 'u' {
			return 0, false
		}
		l.advanceByte()
		lo, ok := l.readHex4()
		if !ok {
			return 0, false
		}
		if lo < 0xDC00 || lo > 0xDFFF {
			return 0, false
		}
		return 0x10000 + (rune(hi)-0xD800)<<10 + (rune(lo) - 0xDC00), true
	}

	if hi >= 0xDC00 && hi <= 0xDFFF {
		return 0, false
	}

	return rune(hi), true
}

func (l *Lexer) readHex4() (int, bool) {
	if l.pos+4 > len(l.input) {
		return 0, false
	}
	v := 0
	for i := 0; i < 4; i++ {
		d := hexVal(l.peekByte())
		if d < 0 {
			return 0, false
		}
		v = v<<4 | d
		l.advanceByte()
	}
	return v, true
}

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	default:
		return -1
	}
}

// scanNumber matches -?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?
func (l *Lexer) scanNumber(line, col int) Token {
	start := l.pos

	if l.peekByte() == '-' {
		l.advanceByte()
	}

	if l.atEnd() || !isDigit(l.peekByte()) {
		return l.numberError(start, line, col)
	}
	if l.peekByte() == '0' {
		l.advanceByte()
		if !l.atEnd() && isDigit(l.peekByte()) {
			return Token{Type: ILLEGAL, Literal: "invalid number: leading zero", Line: line, Col: col}
		}
	} else {
		for !l.atEnd() && isDigit(l.peekByte()) {
			l.advanceByte()
		}
	}

	if !l.atEnd() && l.peekByte() == '.' {
		l.advanceByte()
		if l.atEnd() || !isDigit(l.peekByte()) {
			return Token{Type: ILLEGAL, Literal: "invalid number: missing fraction digits", Line: line, Col: col}
		}
		for !l.atEnd() && isDigit(l.peekByte()) {
			l.advanceByte()
		}
	}

	if !l.atEnd() && (l.peekByte() == 'e' || l.peekByte() == 'E') {
		l.advanceByte()
		if !l.atEnd() && (l.peekByte() == '+' || l.peekByte() == '-') {
			l.advanceByte()
		}
		if l.atEnd() || !isDigit(l.peekByte()) {
			return Token{Type: ILLEGAL, Literal: "invalid number: missing exponent digits", Line: line, Col: col}
		}
		for !l.atEnd() && isDigit(l.peekByte()) {
			l.advanceByte()
		}
	}

	return Token{Type: NUMBER, Literal: string(l.input[start:l.pos]), Line: line, Col: col}
}

// numberError swallows the rest of a malformed number so the message shows
// the whole thing.
func (l *Lexer) numberError(start, line, col int) Token {
	for !l.atEnd() {
		c := l.peekByte()
		if isDigit(c) || c == '.' || c == '-' || c == '+' || c == 'e' || c == 'E' {
			l.advanceByte()
		} else {
			break
		}
	}
	bad := string(l.input[start:l.pos])
	return Token{Type: ILLEGAL, Literal: "invalid number literal \"" + bad + "\"", Line: line, Col: col}
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }
