package jq

import (
	"strings"
	"testing"
)

func TestOrdering(t *testing.T) {
	// null < false < true < numbers < strings < arrays < objects
	vals := []Value{
		nil, false, true, float64(-1), float64(0), float64(5),
		"a", "b", []Value{float64(1)}, []Value{float64(2)},
		mkObj("a", float64(1)),
	}
	for i := 0; i < len(vals)-1; i++ {
		if Compare(vals[i], vals[i+1]) >= 0 {
			t.Errorf("expected %v < %v", vals[i], vals[i+1])
		}
	}
	if !Equal(float64(3), float64(3)) {
		t.Error("3 should equal 3")
	}
	if Equal("a", "b") {
		t.Error("a should not equal b")
	}
}

func mkObj(kv ...Value) *Object {
	o := NewObject()
	for i := 0; i+1 < len(kv); i += 2 {
		o.Set(kv[i].(string), kv[i+1])
	}
	return o
}

func TestEncodeModes(t *testing.T) {
	o := NewObject()
	o.Set("b", float64(2))
	o.Set("a", float64(1))

	if got := Encode(o, EncodeOptions{Compact: true}); got != `{"b":2,"a":1}` {
		t.Errorf("compact insertion order wrong: %s", got)
	}
	if got := Encode(o, EncodeOptions{Compact: true, SortKeys: true}); got != `{"a":1,"b":2}` {
		t.Errorf("sort keys wrong: %s", got)
	}
	pretty := Encode(o, EncodeOptions{})
	if !strings.Contains(pretty, "\n  \"b\": 2") {
		t.Errorf("pretty output wrong:\n%s", pretty)
	}
	if got := Encode("hi", EncodeOptions{Raw: true}); got != "hi" {
		t.Errorf("raw string wrong: %q", got)
	}
	if got := Encode("hi", EncodeOptions{}); got != `"hi"` {
		t.Errorf("quoted string wrong: %q", got)
	}
}

func TestNumberFormat(t *testing.T) {
	cases := map[float64]string{
		0:      "0",
		10:     "10",
		-5:     "-5",
		2.5:    "2.5",
		1000:   "1000",
		3.14:   "3.14",
		-0.001: "-0.001",
	}
	for in, want := range cases {
		if got := FormatNumber(in); got != want {
			t.Errorf("FormatNumber(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestUnicodeEscape(t *testing.T) {
	// ASCII mode escapes non-ASCII as \uXXXX; default keeps the literal rune.
	if got := Encode("é", EncodeOptions{ASCII: true}); got != "\"\\u00e9\"" {
		t.Errorf("ascii escape wrong: %s", got)
	}
	if got := Encode("é", EncodeOptions{}); got != "\"é\"" {
		t.Errorf("non-ascii passthrough wrong: %s", got)
	}
	// Control characters are always escaped.
	if got := Encode("a\tb", EncodeOptions{}); got != `"a\tb"` {
		t.Errorf("tab escape wrong: %s", got)
	}
}

func TestLexerBasics(t *testing.T) {
	toks, err := lex(".foo | .[0] + 1")
	if err != nil {
		t.Fatal(err)
	}
	kinds := []tokKind{}
	for _, tk := range toks {
		kinds = append(kinds, tk.kind)
	}
	want := []tokKind{tField, tPipe, tDot, tLBracket, tNumber, tRBracket, tOp, tNumber, tEOF}
	if len(kinds) != len(want) {
		t.Fatalf("token count: got %v want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Errorf("token %d: got %d want %d", i, kinds[i], want[i])
		}
	}
}

func TestDecodeOrderedObject(t *testing.T) {
	v, err := ParseValue([]byte(`{"z":1,"a":2,"m":3}`))
	if err != nil {
		t.Fatal(err)
	}
	o, ok := v.(*Object)
	if !ok {
		t.Fatalf("expected *Object, got %T", v)
	}
	if strings.Join(o.Keys(), ",") != "z,a,m" {
		t.Errorf("key order not preserved: %v", o.Keys())
	}
}

func TestDecodeRejectsTrailing(t *testing.T) {
	if _, err := ParseValue([]byte(`{"a":1} garbage`)); err == nil {
		t.Error("expected error for trailing data")
	}
}
