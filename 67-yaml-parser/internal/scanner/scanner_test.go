package scanner

import "testing"

func TestSplitLinesIndent(t *testing.T) {
	lines := SplitLines([]byte("a: 1\r\n  b: 2\r\n"))
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0].Indent != 0 || lines[0].Text != "a: 1" {
		t.Errorf("line 0 = %+v", lines[0])
	}
	if lines[1].Indent != 2 || lines[1].Text != "b: 2" {
		t.Errorf("line 1 = %+v", lines[1])
	}
}

func TestSplitLinesTab(t *testing.T) {
	lines := SplitLines([]byte("key:\n\tvalue\n"))
	if !lines[1].HasTabIndent {
		t.Errorf("expected HasTabIndent on tab-indented line")
	}
}

func TestReadDoubleQuoted(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`"hello"`, "hello"},
		{`"a\nb"`, "a\nb"},
		{`"tab\there"`, "tab\there"},
		{`"quote \" inside"`, `quote " inside`},
		{`"slash \/ ok"`, "slash / ok"},
		{`"\x41"`, "A"},
		{`"é"`, "é"},
		{`"back\\slash"`, `back\slash`},
	}
	for _, c := range cases {
		got, ni, err := ReadDoubleQuoted(c.in, 0)
		if err != nil {
			t.Errorf("ReadDoubleQuoted(%q) error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ReadDoubleQuoted(%q) = %q, want %q", c.in, got, c.want)
		}
		if ni != len(c.in) {
			t.Errorf("ReadDoubleQuoted(%q) consumed %d, want %d", c.in, ni, len(c.in))
		}
	}
}

func TestReadDoubleQuotedErrors(t *testing.T) {
	if _, _, err := ReadDoubleQuoted(`"unterminated`, 0); err == nil {
		t.Error("expected unterminated error")
	}
	if _, _, err := ReadDoubleQuoted(`"bad \q"`, 0); err == nil {
		t.Error("expected bad escape error")
	}
}

func TestReadSingleQuoted(t *testing.T) {
	got, _, err := ReadSingleQuoted(`'it''s fine'`, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != "it's fine" {
		t.Errorf("got %q, want %q", got, "it's fine")
	}
	if _, _, err := ReadSingleQuoted(`'unterminated`, 0); err == nil {
		t.Error("expected unterminated error")
	}
}
