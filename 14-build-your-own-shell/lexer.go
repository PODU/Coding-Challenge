package main

import (
	"fmt"
	"strings"
)

type TokenType int

const (
	TokenWord           TokenType = iota // an argument, quotes already removed
	TokenPipe                            // |
	TokenRedirectOut                     // >
	TokenRedirectAppend                  // >>
	TokenRedirectIn                      // <
)

func (t TokenType) String() string {
	switch t {
	case TokenWord:
		return "word"
	case TokenPipe:
		return "|"
	case TokenRedirectOut:
		return ">"
	case TokenRedirectAppend:
		return ">>"
	case TokenRedirectIn:
		return "<"
	default:
		return "unknown"
	}
}

type Token struct {
	Type  TokenType
	Value string
}

// SyntaxError is returned for a line that can't be tokenized or parsed.
type SyntaxError struct {
	Msg string
}

func (e *SyntaxError) Error() string { return "syntax error: " + e.Msg }

// Tokenize splits a command line into words and the operators | > >> <,
// handling single quotes, double quotes and backslash escapes.
func Tokenize(line string) ([]Token, error) {
	var tokens []Token
	var buf strings.Builder
	wordActive := false

	runes := []rune(line)
	flush := func() {
		if wordActive {
			tokens = append(tokens, Token{Type: TokenWord, Value: buf.String()})
			buf.Reset()
			wordActive = false
		}
	}

	for i := 0; i < len(runes); {
		c := runes[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			flush()
			i++

		case c == '|':
			flush()
			tokens = append(tokens, Token{Type: TokenPipe, Value: "|"})
			i++

		case c == '<':
			flush()
			tokens = append(tokens, Token{Type: TokenRedirectIn, Value: "<"})
			i++

		case c == '>':
			flush()
			if i+1 < len(runes) && runes[i+1] == '>' {
				tokens = append(tokens, Token{Type: TokenRedirectAppend, Value: ">>"})
				i += 2
			} else {
				tokens = append(tokens, Token{Type: TokenRedirectOut, Value: ">"})
				i++
			}

		case c == '\'':
			// Nothing is special inside single quotes.
			wordActive = true
			i++
			start := i
			for i < len(runes) && runes[i] != '\'' {
				i++
			}
			if i >= len(runes) {
				return nil, &SyntaxError{Msg: "unterminated single quote"}
			}
			buf.WriteString(string(runes[start:i]))
			i++

		case c == '"':
			// Inside double quotes a backslash only escapes " and \.
			wordActive = true
			i++
			for i < len(runes) && runes[i] != '"' {
				if runes[i] == '\\' && i+1 < len(runes) {
					next := runes[i+1]
					if next == '"' || next == '\\' {
						buf.WriteRune(next)
						i += 2
						continue
					}
				}
				buf.WriteRune(runes[i])
				i++
			}
			if i >= len(runes) {
				return nil, &SyntaxError{Msg: "unterminated double quote"}
			}
			i++

		case c == '\\':
			if i+1 >= len(runes) {
				return nil, &SyntaxError{Msg: "trailing backslash"}
			}
			wordActive = true
			buf.WriteRune(runes[i+1])
			i += 2

		default:
			wordActive = true
			buf.WriteRune(c)
			i++
		}
	}

	flush()
	return tokens, nil
}

// describe formats tokens for test failure messages.
func describe(tokens []Token) string {
	parts := make([]string, len(tokens))
	for i, t := range tokens {
		parts[i] = fmt.Sprintf("%s(%q)", t.Type, t.Value)
	}
	return strings.Join(parts, " ")
}
