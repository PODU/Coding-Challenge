package main

// Token is a command and its byte offset in the source, kept for error
// messages.
type Token struct {
	Cmd    byte
	Offset int
}

func isCommand(b byte) bool {
	switch b {
	case '>', '<', '+', '-', '.', ',', '[', ']':
		return true
	default:
		return false
	}
}

// Tokenize keeps the eight command bytes and drops everything else, which
// Brainfuck treats as comments.
func Tokenize(src []byte) []Token {
	tokens := make([]Token, 0, len(src))
	for i := 0; i < len(src); i++ {
		if isCommand(src[i]) {
			tokens = append(tokens, Token{Cmd: src[i], Offset: i})
		}
	}
	return tokens
}
