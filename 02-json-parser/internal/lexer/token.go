package lexer

// TokenType is the kind of a lexical token.
type TokenType int

const (
	// ILLEGAL marks input the lexer could not tokenize.
	ILLEGAL TokenType = iota
	EOF

	LBRACE   // {
	RBRACE   // }
	LBRACKET // [
	RBRACKET // ]
	COLON    // :
	COMMA    // ,

	STRING // "..."
	NUMBER // 123, -1.2e+10
	TRUE   // true
	FALSE  // false
	NULL   // null
)

func (t TokenType) String() string {
	switch t {
	case ILLEGAL:
		return "ILLEGAL"
	case EOF:
		return "EOF"
	case LBRACE:
		return "LBRACE"
	case RBRACE:
		return "RBRACE"
	case LBRACKET:
		return "LBRACKET"
	case RBRACKET:
		return "RBRACKET"
	case COLON:
		return "COLON"
	case COMMA:
		return "COMMA"
	case STRING:
		return "STRING"
	case NUMBER:
		return "NUMBER"
	case TRUE:
		return "TRUE"
	case FALSE:
		return "FALSE"
	case NULL:
		return "NULL"
	default:
		return "UNKNOWN"
	}
}

// Token is one lexical unit. Line and Col are 1-based.
//
// Literal depends on Type: the decoded value for STRING, the source text for
// NUMBER, and the error reason for ILLEGAL.
type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Col     int
}
