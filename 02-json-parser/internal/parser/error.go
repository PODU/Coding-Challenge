package parser

import "fmt"

// ParseError reports a lexical or syntax error at a 1-based position.
type ParseError struct {
	Message string
	Line    int
	Col     int
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("%s at line %d, column %d", e.Message, e.Line, e.Col)
}
