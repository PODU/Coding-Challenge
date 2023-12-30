package set

import (
	"bytes"
	"testing"
)

func mustExpand(t *testing.T, s string, target int) []byte {
	t.Helper()
	st, err := Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q) error: %v", s, err)
	}
	out, err := Expand(st, target)
	if err != nil {
		t.Fatalf("Expand(%q) error: %v", s, err)
	}
	return out
}

func TestLiteralAndEscapes(t *testing.T) {
	cases := []struct {
		in   string
		want []byte
	}{
		{"abc", []byte("abc")},
		{`\n`, []byte{'\n'}},
		{`\t`, []byte{'\t'}},
		{`\\`, []byte{'\\'}},
		{`\101`, []byte{'A'}}, // octal 101 == 'A'
		{`\0`, []byte{0}},
		{`a\-b`, []byte{'a', '-', 'b'}}, // \- is a literal dash
		{`x\`, []byte{'x', '\\'}},       // lone trailing backslash
	}
	for _, c := range cases {
		got := mustExpand(t, c.in, -1)
		if !bytes.Equal(got, c.want) {
			t.Errorf("Expand(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestRanges(t *testing.T) {
	if got := mustExpand(t, "a-e", -1); string(got) != "abcde" {
		t.Errorf("a-e = %q", got)
	}
	if got := mustExpand(t, "0-9a-f", -1); string(got) != "0123456789abcdef" {
		t.Errorf("0-9a-f = %q", got)
	}
	// Leading and trailing dash are literal.
	if got := mustExpand(t, "-a", -1); string(got) != "-a" {
		t.Errorf("-a = %q", got)
	}
	if got := mustExpand(t, "a-", -1); string(got) != "a-" {
		t.Errorf("a- = %q", got)
	}
	// Single-element range.
	if got := mustExpand(t, "a-a", -1); string(got) != "a" {
		t.Errorf("a-a = %q", got)
	}
}

func TestReversedRangeError(t *testing.T) {
	if _, err := Parse("z-a"); err == nil {
		t.Error("expected error for reversed range z-a")
	}
}

func TestClasses(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"[:digit:]", "0123456789"},
		{"[:lower:]", "abcdefghijklmnopqrstuvwxyz"},
		{"[:upper:]", "ABCDEFGHIJKLMNOPQRSTUVWXYZ"},
		{"[:xdigit:]", "0123456789ABCDEFabcdef"},
		{"[:blank:]", "\t "},
		{"[:space:]", "\t\n\v\f\r "},
	}
	for _, c := range cases {
		got := mustExpand(t, c.in, -1)
		if string(got) != c.want {
			t.Errorf("Expand(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestClassMemberCounts(t *testing.T) {
	if got := mustExpand(t, "[:alpha:]", -1); len(got) != 52 {
		t.Errorf("alpha len = %d, want 52", len(got))
	}
	if got := mustExpand(t, "[:cntrl:]", -1); len(got) != 33 {
		t.Errorf("cntrl len = %d, want 33", len(got))
	}
	if got := mustExpand(t, "[:print:]", -1); len(got) != 95 {
		t.Errorf("print len = %d, want 95", len(got))
	}
	if got := mustExpand(t, "[:graph:]", -1); len(got) != 94 {
		t.Errorf("graph len = %d, want 94", len(got))
	}
}

func TestUnknownClassError(t *testing.T) {
	if _, err := Parse("[:nope:]"); err == nil {
		t.Error("expected error for unknown class")
	}
}

func TestRepeat(t *testing.T) {
	// Exact repeat.
	if got := mustExpand(t, "[x*3]", -1); string(got) != "xxx" {
		t.Errorf("[x*3] = %q", got)
	}
	// Octal count (leading zero).
	if got := mustExpand(t, "[x*010]", -1); len(got) != 8 {
		t.Errorf("[x*010] len = %d, want 8", len(got))
	}
	// Pad to target length.
	if got := mustExpand(t, "ab[z*]", 5); string(got) != "abzzz" {
		t.Errorf("ab[z*] target 5 = %q", got)
	}
	// Pad construct illegal in SET1 (target -1).
	if st, _ := Parse("[z*]"); func() bool { _, err := Expand(st, -1); return err == nil }() {
		t.Error("expected error: [z*] in SET1")
	}
}

func TestEquivalence(t *testing.T) {
	if got := mustExpand(t, "[=a=]", -1); string(got) != "a" {
		t.Errorf("[=a=] = %q", got)
	}
}

func TestSet2ClassesOK(t *testing.T) {
	if st, _ := Parse("[:upper:]"); st.Set2ClassesOK() != nil {
		t.Error("upper should be allowed in SET2")
	}
	if st, _ := Parse("[:digit:]"); st.Set2ClassesOK() == nil {
		t.Error("digit should be rejected in SET2")
	}
}

func TestBracketLiteral(t *testing.T) {
	// Brackets that are not a recognized construct are literal.
	if got := mustExpand(t, "[a-c]", -1); string(got) != "[abc]" {
		t.Errorf("[a-c] = %q, want [abc]", got)
	}
}
