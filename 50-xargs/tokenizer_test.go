package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestTokenizeWhitespace(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"spaces", "a b c", []string{"a", "b", "c"}},
		{"mixed blanks", "a b\tc\nd", []string{"a", "b", "c", "d"}},
		{"collapse runs", "a   b\t\t c", []string{"a", "b", "c"}},
		{"leading/trailing", "  a b  ", []string{"a", "b"}},
		{"crlf", "a\r\nb\r\nc", []string{"a", "b", "c"}},
		{"empty", "", nil},
		{"only blanks", "   \t\n ", nil},
		{"single quotes", "'a b' c", []string{"a b", "c"}},
		{"double quotes", "\"a b\" c", []string{"a b", "c"}},
		{"backslash escape", `a\ b`, []string{"a b"}},
		{"quote joins token", "foo'bar baz'qux end", []string{"foobar bazqux", "end"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TokenizeAll(strings.NewReader(tt.in), TokenizerConfig{Mode: ModeWhitespace})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestTokenizeUnbalancedQuote(t *testing.T) {
	_, err := TokenizeAll(strings.NewReader("'unterminated"), TokenizerConfig{Mode: ModeWhitespace})
	if err == nil {
		t.Fatal("expected an error for unbalanced quote, got nil")
	}
}

func TestTokenizeNull(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"basic", "a\x00b\x00", []string{"a", "b"}},
		{"no trailing nul", "a\x00b", []string{"a", "b"}},
		{"spaces preserved", "a b\x00c", []string{"a b", "c"}},
		{"empty", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TokenizeAll(strings.NewReader(tt.in), TokenizerConfig{Mode: ModeNull})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestTokenizeDelimiter(t *testing.T) {
	tests := []struct {
		name  string
		delim byte
		in    string
		want  []string
	}{
		{"comma with empty field", ',', "a,b,,c", []string{"a", "b", "", "c"}},
		{"trailing delim ignored", ',', "a,b,", []string{"a", "b"}},
		{"leading empty field", ',', ",a", []string{"", "a"}},
		{"newline delim", '\n', "a\nb\nc", []string{"a", "b", "c"}},
		{"spaces preserved", ',', "a b,c d", []string{"a b", "c d"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TokenizeAll(strings.NewReader(tt.in),
				TokenizerConfig{Mode: ModeDelimiter, Delimiter: tt.delim})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestTokenizeEOFString(t *testing.T) {
	got, err := TokenizeAll(strings.NewReader("a b EOF c d"),
		TokenizerConfig{Mode: ModeWhitespace, EOFString: "EOF", EOFSet: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestParseDelimiterEscapes(t *testing.T) {
	cases := map[string]byte{
		",":    ',',
		"\\n":  '\n',
		"\\t":  '\t',
		"\\r":  '\r',
		"\\0":  0,
		"\\\\": '\\',
	}
	for in, want := range cases {
		got, err := parseDelimiter(in)
		if err != nil {
			t.Fatalf("parseDelimiter(%q) error: %v", in, err)
		}
		if got != want {
			t.Fatalf("parseDelimiter(%q) = %d, want %d", in, got, want)
		}
	}
	if _, err := parseDelimiter("ab"); err == nil {
		t.Fatal("expected error for multi-char delimiter")
	}
}
