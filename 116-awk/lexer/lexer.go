// Package lexer turns awk source text into a stream of tokens.
package lexer

import (
	"fmt"
	"strings"
)

// Type is a token type.
type Type int

// Token types.
const (
	ILLEGAL Type = iota
	EOF
	NEWLINE

	NUMBER
	STRING
	REGEX
	NAME      // identifier
	FUNC_NAME // identifier immediately followed by '('

	// keywords
	BEGIN
	END
	FUNCTION
	IF
	ELSE
	WHILE
	FOR
	DO
	BREAK
	CONTINUE
	NEXT
	NEXTFILE
	EXIT
	RETURN
	DELETE
	IN
	GETLINE
	PRINT
	PRINTF

	// assignment operators
	ASSIGN
	ADD_ASSIGN
	SUB_ASSIGN
	MUL_ASSIGN
	DIV_ASSIGN
	MOD_ASSIGN
	POW_ASSIGN

	// comparison
	EQ
	NE
	LT
	LE
	GT
	GE

	// match
	MATCH
	NOTMATCH

	// logical
	NOT
	AND
	OR

	// arithmetic
	ADD
	SUB
	MUL
	DIV
	MOD
	POW

	INCR
	DECR

	DOLLAR

	LPAREN
	RPAREN
	LBRACE
	RBRACE
	LBRACKET
	RBRACKET

	SEMI
	COMMA
	QUESTION
	COLON

	APPEND // >>
	PIPE   // |
)

var typeNames = map[Type]string{
	ILLEGAL: "ILLEGAL", EOF: "EOF", NEWLINE: "NEWLINE",
	NUMBER: "NUMBER", STRING: "STRING", REGEX: "REGEX", NAME: "NAME", FUNC_NAME: "FUNC_NAME",
	BEGIN: "BEGIN", END: "END", FUNCTION: "function", IF: "if", ELSE: "else",
	WHILE: "while", FOR: "for", DO: "do", BREAK: "break", CONTINUE: "continue",
	NEXT: "next", NEXTFILE: "nextfile", EXIT: "exit", RETURN: "return",
	DELETE: "delete", IN: "in", GETLINE: "getline", PRINT: "print", PRINTF: "printf",
	ASSIGN: "=", ADD_ASSIGN: "+=", SUB_ASSIGN: "-=", MUL_ASSIGN: "*=",
	DIV_ASSIGN: "/=", MOD_ASSIGN: "%=", POW_ASSIGN: "^=",
	EQ: "==", NE: "!=", LT: "<", LE: "<=", GT: ">", GE: ">=",
	MATCH: "~", NOTMATCH: "!~", NOT: "!", AND: "&&", OR: "||",
	ADD: "+", SUB: "-", MUL: "*", DIV: "/", MOD: "%", POW: "^",
	INCR: "++", DECR: "--", DOLLAR: "$",
	LPAREN: "(", RPAREN: ")", LBRACE: "{", RBRACE: "}",
	LBRACKET: "[", RBRACKET: "]", SEMI: ";", COMMA: ",",
	QUESTION: "?", COLON: ":", APPEND: ">>", PIPE: "|",
}

// String returns a human-readable name for the token type.
func (t Type) String() string {
	if s, ok := typeNames[t]; ok {
		return s
	}
	return fmt.Sprintf("Type(%d)", int(t))
}

// Token is a single lexical token.
type Token struct {
	Type    Type
	Literal string // decoded value for STRING/REGEX/NUMBER/NAME
	Line    int
}

var keywords = map[string]Type{
	"BEGIN": BEGIN, "END": END, "function": FUNCTION, "func": FUNCTION,
	"if": IF, "else": ELSE, "while": WHILE, "for": FOR, "do": DO,
	"break": BREAK, "continue": CONTINUE, "next": NEXT, "nextfile": NEXTFILE,
	"exit": EXIT, "return": RETURN, "delete": DELETE, "in": IN,
	"getline": GETLINE, "print": PRINT, "printf": PRINTF,
}

// Error is a lexer error with position information.
type Error struct {
	Line int
	Msg  string
}

func (e *Error) Error() string { return fmt.Sprintf("line %d: %s", e.Line, e.Msg) }

type lexer struct {
	src  string
	pos  int
	line int
	toks []Token
	last Type // last appended token type
}

// Tokenize converts source text into a slice of tokens terminated by EOF.
func Tokenize(src string) ([]Token, error) {
	l := &lexer{src: src, line: 1, last: ILLEGAL}
	if err := l.run(); err != nil {
		return nil, err
	}
	return l.toks, nil
}

func (l *lexer) emit(t Type, lit string) {
	l.toks = append(l.toks, Token{Type: t, Literal: lit, Line: l.line})
	l.last = t
}

func (l *lexer) peekCh() byte {
	if l.pos < len(l.src) {
		return l.src[l.pos]
	}
	return 0
}

func (l *lexer) peekCh2() byte {
	if l.pos+1 < len(l.src) {
		return l.src[l.pos+1]
	}
	return 0
}

func (l *lexer) run() error {
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case c == ' ' || c == '\t' || c == '\r':
			l.pos++
		case c == '\\' && l.peekCh2() == '\n':
			l.pos += 2
			l.line++
		case c == '\\' && l.peekCh2() == '\r':
			// line continuation with CRLF
			l.pos += 2
			if l.peekCh() == '\n' {
				l.pos++
			}
			l.line++
		case c == '#':
			for l.pos < len(l.src) && l.src[l.pos] != '\n' {
				l.pos++
			}
		case c == '\n':
			l.pos++
			l.maybeNewline()
			l.line++
		case isDigit(c) || (c == '.' && isDigit(l.peekCh2())):
			l.lexNumber()
		case isAlpha(c):
			l.lexName()
		case c == '"':
			if err := l.lexString(); err != nil {
				return err
			}
		case c == '/':
			if l.regexAllowed() {
				if err := l.lexRegex(); err != nil {
					return err
				}
			} else {
				l.lexOperator()
			}
		default:
			if err := l.lexOperator(); err != nil {
				return err
			}
		}
	}
	l.emit(EOF, "")
	return nil
}

// maybeNewline emits a NEWLINE unless the previous token lets a statement
// continue onto the next line (after "&&", ",", "{", etc.).
func (l *lexer) maybeNewline() {
	switch l.last {
	case ILLEGAL, NEWLINE, LBRACE, AND, OR, COMMA, SEMI, DO, ELSE, QUESTION, COLON:
	default:
		l.emit(NEWLINE, "")
	}
}

// regexAllowed reports whether a '/' at the current position begins a regex
// literal (true) or is the division operator (false).
func (l *lexer) regexAllowed() bool {
	switch l.last {
	case NAME, NUMBER, STRING, RPAREN, RBRACKET, DOLLAR, INCR, DECR:
		return false
	default:
		return true
	}
}

func (l *lexer) lexNumber() {
	start := l.pos
	for l.pos < len(l.src) && isDigit(l.src[l.pos]) {
		l.pos++
	}
	if l.peekCh() == '.' {
		l.pos++
		for l.pos < len(l.src) && isDigit(l.src[l.pos]) {
			l.pos++
		}
	}
	if c := l.peekCh(); c == 'e' || c == 'E' {
		save := l.pos
		l.pos++
		if c := l.peekCh(); c == '+' || c == '-' {
			l.pos++
		}
		if isDigit(l.peekCh()) {
			for l.pos < len(l.src) && isDigit(l.src[l.pos]) {
				l.pos++
			}
		} else {
			l.pos = save // not an exponent
		}
	}
	l.emit(NUMBER, l.src[start:l.pos])
}

func (l *lexer) lexName() {
	start := l.pos
	for l.pos < len(l.src) && (isAlpha(l.src[l.pos]) || isDigit(l.src[l.pos])) {
		l.pos++
	}
	word := l.src[start:l.pos]
	if kw, ok := keywords[word]; ok {
		l.emit(kw, word)
		return
	}
	if l.peekCh() == '(' {
		l.emit(FUNC_NAME, word)
		return
	}
	l.emit(NAME, word)
}

func (l *lexer) lexString() error {
	l.pos++ // opening quote
	var b strings.Builder
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		if c == '"' {
			l.pos++
			l.emit(STRING, b.String())
			return nil
		}
		if c == '\n' {
			return &Error{l.line, "newline in string"}
		}
		if c == '\\' {
			l.pos++
			if l.pos >= len(l.src) {
				break
			}
			e := l.src[l.pos]
			switch e {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '\\':
				b.WriteByte('\\')
			case '"':
				b.WriteByte('"')
			case '/':
				b.WriteByte('/')
			case 'a':
				b.WriteByte('\a')
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case 'v':
				b.WriteByte('\v')
			case '0', '1', '2', '3', '4', '5', '6', '7':
				// up to 3 octal digits
				val := 0
				n := 0
				for n < 3 && l.pos < len(l.src) && l.src[l.pos] >= '0' && l.src[l.pos] <= '7' {
					val = val*8 + int(l.src[l.pos]-'0')
					l.pos++
					n++
				}
				b.WriteByte(byte(val))
				continue
			default:
				b.WriteByte('\\')
				b.WriteByte(e)
			}
			l.pos++
			continue
		}
		b.WriteByte(c)
		l.pos++
	}
	return &Error{l.line, "unterminated string literal"}
}

func (l *lexer) lexRegex() error {
	l.pos++ // opening slash
	var b strings.Builder
	inClass := false
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		switch {
		case c == '\n':
			return &Error{l.line, "newline in regex literal"}
		case c == '\\':
			if l.pos+1 < len(l.src) {
				nxt := l.src[l.pos+1]
				if nxt == '/' {
					b.WriteByte('/')
				} else {
					b.WriteByte('\\')
					b.WriteByte(nxt)
				}
				l.pos += 2
				continue
			}
			b.WriteByte('\\')
			l.pos++
		case c == '[':
			inClass = true
			b.WriteByte(c)
			l.pos++
		case c == ']':
			inClass = false
			b.WriteByte(c)
			l.pos++
		case c == '/' && !inClass:
			l.pos++
			l.emit(REGEX, b.String())
			return nil
		default:
			b.WriteByte(c)
			l.pos++
		}
	}
	return &Error{l.line, "unterminated regex literal"}
}

func (l *lexer) lexOperator() error {
	c := l.src[l.pos]
	two := func(t Type) {
		l.pos += 2
		l.emit(t, "")
	}
	one := func(t Type) {
		l.pos++
		l.emit(t, "")
	}
	n := l.peekCh2()
	switch c {
	case '+':
		switch n {
		case '+':
			two(INCR)
		case '=':
			two(ADD_ASSIGN)
		default:
			one(ADD)
		}
	case '-':
		switch n {
		case '-':
			two(DECR)
		case '=':
			two(SUB_ASSIGN)
		default:
			one(SUB)
		}
	case '*':
		switch {
		case n == '*' && l.peekAt(2) == '=':
			l.pos += 3
			l.emit(POW_ASSIGN, "")
		case n == '*':
			two(POW)
		case n == '=':
			two(MUL_ASSIGN)
		default:
			one(MUL)
		}
	case '/':
		if n == '=' {
			two(DIV_ASSIGN)
		} else {
			one(DIV)
		}
	case '%':
		if n == '=' {
			two(MOD_ASSIGN)
		} else {
			one(MOD)
		}
	case '^':
		if n == '=' {
			two(POW_ASSIGN)
		} else {
			one(POW)
		}
	case '=':
		if n == '=' {
			two(EQ)
		} else {
			one(ASSIGN)
		}
	case '!':
		switch n {
		case '=':
			two(NE)
		case '~':
			two(NOTMATCH)
		default:
			one(NOT)
		}
	case '<':
		if n == '=' {
			two(LE)
		} else {
			one(LT)
		}
	case '>':
		switch n {
		case '=':
			two(GE)
		case '>':
			two(APPEND)
		default:
			one(GT)
		}
	case '~':
		one(MATCH)
	case '&':
		if n == '&' {
			two(AND)
		} else {
			return &Error{l.line, "unexpected '&'"}
		}
	case '|':
		if n == '|' {
			two(OR)
		} else {
			one(PIPE)
		}
	case '?':
		one(QUESTION)
	case ':':
		one(COLON)
	case '$':
		one(DOLLAR)
	case '(':
		one(LPAREN)
	case ')':
		one(RPAREN)
	case '{':
		one(LBRACE)
	case '}':
		one(RBRACE)
	case '[':
		one(LBRACKET)
	case ']':
		one(RBRACKET)
	case ';':
		one(SEMI)
	case ',':
		one(COMMA)
	default:
		return &Error{l.line, fmt.Sprintf("unexpected character %q", string(c))}
	}
	return nil
}

func (l *lexer) peekAt(off int) byte {
	if l.pos+off < len(l.src) {
		return l.src[l.pos+off]
	}
	return 0
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
func isAlpha(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
