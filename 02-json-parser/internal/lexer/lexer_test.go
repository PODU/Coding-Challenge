package lexer

import "testing"

// collect drains a lexer into a slice of tokens (including the final EOF).
func collect(input string) []Token {
	l := New([]byte(input))
	var toks []Token
	for {
		t := l.NextToken()
		toks = append(toks, t)
		if t.Type == EOF || t.Type == ILLEGAL {
			break
		}
	}
	return toks
}

func TestStructuralTokens(t *testing.T) {
	toks := collect("{}[]:,")
	want := []TokenType{LBRACE, RBRACE, LBRACKET, RBRACKET, COLON, COMMA, EOF}
	if len(toks) != len(want) {
		t.Fatalf("got %d tokens, want %d: %v", len(toks), len(want), toks)
	}
	for i, w := range want {
		if toks[i].Type != w {
			t.Errorf("token %d: got %v, want %v", i, toks[i].Type, w)
		}
	}
}

func TestKeywordTokens(t *testing.T) {
	cases := map[string]TokenType{
		"true":  TRUE,
		"false": FALSE,
		"null":  NULL,
	}
	for in, want := range cases {
		toks := collect(in)
		if toks[0].Type != want {
			t.Errorf("%q: got %v, want %v", in, toks[0].Type, want)
		}
	}
}

func TestInvalidKeywords(t *testing.T) {
	for _, in := range []string{"tru", "False", "nul", "TRUE", "nil"} {
		toks := collect(in)
		if toks[0].Type != ILLEGAL {
			t.Errorf("%q: expected ILLEGAL, got %v", in, toks[0].Type)
		}
	}
}

func TestStringDecoding(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`"hello"`, "hello"},
		{`""`, ""},
		{`"a\"b"`, `a"b`},
		{`"a\\b"`, `a\b`},
		{`"\/"`, "/"},
		{`"\b\f\n\r\t"`, "\b\f\n\r\t"},
		{`"A"`, "A"},
		{`"é"`, "é"},
		{`"😀"`, "\U0001F600"}, // surrogate pair -> emoji
		{`"café"`, "café"},
	}
	for _, c := range cases {
		toks := collect(c.in)
		if toks[0].Type != STRING {
			t.Errorf("%q: got %v, want STRING (%q)", c.in, toks[0].Type, toks[0].Literal)
			continue
		}
		if toks[0].Literal != c.want {
			t.Errorf("%q: decoded %q, want %q", c.in, toks[0].Literal, c.want)
		}
	}
}

func TestInvalidStrings(t *testing.T) {
	cases := []string{
		`"unterminated`,
		`"bad\xescape"`,
		`"short\u12"`,
		"\"raw\tcontrol\"",
		`"\uD83D"`,  // lone high surrogate
		`"\uDE00"`,  // lone low surrogate
		`"\uD83Dx"`, // high surrogate not followed by \u
		`"\uD83DA"`, // high surrogate followed by non-low-surrogate
	}
	for _, in := range cases {
		toks := collect(in)
		if toks[len(toks)-1].Type != ILLEGAL {
			t.Errorf("%q: expected ILLEGAL, got %v", in, toks[len(toks)-1].Type)
		}
	}
}

func TestValidNumbers(t *testing.T) {
	cases := []string{
		"0", "-0", "1", "-1", "123", "0.5", "-0.5", "3.14159",
		"1e10", "1E10", "1e+10", "1e-10", "-1.2e+10", "0e0", "1.0e0",
		"123456789012345678901234567890",
	}
	for _, in := range cases {
		toks := collect(in)
		if toks[0].Type != NUMBER {
			t.Errorf("%q: got %v, want NUMBER", in, toks[0].Type)
			continue
		}
		if toks[0].Literal != in {
			t.Errorf("%q: literal %q", in, toks[0].Literal)
		}
	}
}

func TestInvalidNumbers(t *testing.T) {
	cases := []string{
		"01", "00", "-01", "1.", ".5", "+1", "1e", "1e+", "1e-",
		"1.e5", "0x1", "--1", "1..2",
	}
	for _, in := range cases {
		toks := collect(in)
		// A bad number surfaces as ILLEGAL somewhere in the stream.
		ok := false
		for _, tk := range toks {
			if tk.Type == ILLEGAL {
				ok = true
				break
			}
		}
		if !ok {
			t.Errorf("%q: expected ILLEGAL token, got %v", in, toks)
		}
	}
}

func TestPositions(t *testing.T) {
	l := New([]byte("{\n  \"k\": 1\n}"))
	tok := l.NextToken() // {
	if tok.Line != 1 || tok.Col != 1 {
		t.Errorf("'{' at line %d col %d, want 1,1", tok.Line, tok.Col)
	}
	tok = l.NextToken() // "k" on line 2, col 3
	if tok.Type != STRING || tok.Line != 2 || tok.Col != 3 {
		t.Errorf(`"k" at %v line %d col %d, want STRING 2,3`, tok.Type, tok.Line, tok.Col)
	}
}
