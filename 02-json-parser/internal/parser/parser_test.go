package parser

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidDocuments(t *testing.T) {
	cases := []string{
		`{}`,
		`[]`,
		`{"key":"value"}`,
		`{"k1":"v1","k2":"v2"}`,
		`{"key1": true, "key2": false, "key3": null, "key4": "value", "key5": 101}`,
		`{"a":{"b":{"c":[1,2,3]}}}`,
		`[1, 2.5, -3, true, false, null, "x", {}, []]`,
		`42`,   // top-level scalar (RFC 8259)
		`"hi"`, // top-level string
		`true`,
		`null`,
		`-1.2e+10`,
		`  {  }  `,      // surrounding whitespace
		"\n\t[]\r\n",    // whitespace variety
		`{"a":1,"a":2}`, // duplicate keys allowed
		`["😀","😀"]`,
	}
	for _, in := range cases {
		if _, err := Parse([]byte(in)); err != nil {
			t.Errorf("Parse(%q) = error %v, want valid", in, err)
		}
		// Oracle cross-check against the stdlib.
		if !json.Valid([]byte(in)) {
			t.Errorf("oracle: stdlib rejects %q that we accept", in)
		}
	}
}

func TestInvalidDocuments(t *testing.T) {
	cases := []string{
		``,    // empty
		`   `, // whitespace only
		`{`,
		`}`,
		`{,}`,
		`{"key":"value",}`, // trailing comma
		`{key:"value"}`,    // unquoted key
		`{"key" "value"}`,  // missing colon
		`{"a":1 "b":2}`,    // missing comma
		`[1,2,]`,           // trailing comma in array
		`[,]`,
		`[1,,2]`,
		`{"a":}`, // missing value
		`{"a":1,}`,
		`{"key": False}`,
		`{"key": tru}`,
		`{"n": 01}`,
		`{"n": 1.}`,
		`{"n": .5}`,
		`{"n": +1}`,
		`{"n": 1e}`,
		`["list value"}`, // mismatched bracket
		`{"a":[}`,
		`[1,2,3`, // unbalanced
		`{}x`,    // trailing data
		`{} {}`,
		`["unterminated]`,
		`nul`,
		`tru`,
	}
	for _, in := range cases {
		if _, err := Parse([]byte(in)); err == nil {
			t.Errorf("Parse(%q) = nil error, want invalid", in)
		}
	}
}

func TestErrorMessages(t *testing.T) {
	cases := []struct {
		in       string
		contains string
	}{
		{`{"key":"value",}`, "trailing comma"},
		{`{key:"value"}`, "expected '\"' to start object key"},
		{`{"key" "value"}`, "expected ':' after object key"},
		{`{"n": 01}`, "leading zero"},
		{`"unterminated`, "unterminated string"},
		{`{}x`, "trailing data"},
		{``, "empty document"},
		{`{"a":1,}`, "trailing comma"},
		{`[1 2]`, "expected ',' or ']'"},
	}
	for _, c := range cases {
		_, err := Parse([]byte(c.in))
		if err == nil {
			t.Errorf("Parse(%q): expected error containing %q, got nil", c.in, c.contains)
			continue
		}
		if !strings.Contains(err.Error(), c.contains) {
			t.Errorf("Parse(%q): error %q does not contain %q", c.in, err.Error(), c.contains)
		}
	}
}

func TestErrorPositions(t *testing.T) {
	_, err := Parse([]byte("{\n  \"a\": tru\n}"))
	if err == nil {
		t.Fatal("expected error")
	}
	pe, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("expected *ParseError, got %T", err)
	}
	if pe.Line != 2 {
		t.Errorf("error at line %d, want 2", pe.Line)
	}
	// Message should carry "at line L, column C".
	if !strings.Contains(pe.Error(), "line 2") {
		t.Errorf("formatted error %q missing line info", pe.Error())
	}
}

func TestMaxDepthGuard(t *testing.T) {
	deep := strings.Repeat("[", 50) + strings.Repeat("]", 50)
	// Default depth allows this.
	if _, err := Parse([]byte(deep)); err != nil {
		t.Errorf("depth 50 should be allowed by default: %v", err)
	}
	// With a small guard it should be rejected.
	if _, err := Parse([]byte(deep), WithMaxDepth(10)); err == nil {
		t.Error("expected depth-limit error with WithMaxDepth(10)")
	}
}

func TestASTShape(t *testing.T) {
	v, err := Parse([]byte(`{"n":42,"s":"x","arr":[true,null]}`))
	if err != nil {
		t.Fatal(err)
	}
	obj, ok := v.(*Object)
	if !ok {
		t.Fatalf("top-level is %T, want *Object", v)
	}
	if len(obj.Members) != 3 {
		t.Fatalf("got %d members, want 3", len(obj.Members))
	}
	num, ok := obj.Members[0].Value.(*Number)
	if !ok || num.ParsedFloat() != 42 {
		t.Errorf("first member not number 42: %+v", obj.Members[0])
	}
}
