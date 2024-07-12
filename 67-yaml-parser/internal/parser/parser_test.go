package parser

import (
	"testing"

	"yamlcc/internal/value"
)

func parseOne(t *testing.T, src string) value.Value {
	t.Helper()
	docs, err := Parse([]byte(src), false)
	if err != nil {
		t.Fatalf("parse %q: %v", src, err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}
	return docs[0]
}

func TestFlowNesting(t *testing.T) {
	v := parseOne(t, "root: [1, [2, 3], {a: 4}]\n")
	root := v.Items[0].Val
	if root.Kind != value.Seq || len(root.Seq) != 3 {
		t.Fatalf("expected seq of 3, got %+v", root)
	}
	inner := root.Seq[1]
	if inner.Kind != value.Seq || inner.Seq[0].Int != 2 {
		t.Errorf("inner seq wrong: %+v", inner)
	}
	m := root.Seq[2]
	if m.Kind != value.Map || m.Items[0].Key != "a" || m.Items[0].Val.Int != 4 {
		t.Errorf("inner map wrong: %+v", m)
	}
}

func TestFlowEmptyAndTrailingComma(t *testing.T) {
	v := parseOne(t, "a: []\nb: {}\nc: [1, 2,]\n")
	if v.Items[0].Val.Kind != value.Seq || len(v.Items[0].Val.Seq) != 0 {
		t.Error("a should be empty seq")
	}
	if v.Items[1].Val.Kind != value.Map || len(v.Items[1].Val.Items) != 0 {
		t.Error("b should be empty map")
	}
	if len(v.Items[2].Val.Seq) != 2 {
		t.Error("c should have 2 items (trailing comma ignored)")
	}
}

func TestBlockLiteralAndFolded(t *testing.T) {
	lit := parseOne(t, "x: |\n  a\n  b\n").Items[0].Val
	if lit.Str != "a\nb\n" {
		t.Errorf("literal = %q, want %q", lit.Str, "a\nb\n")
	}
	fold := parseOne(t, "x: >\n  a\n  b\n").Items[0].Val
	if fold.Str != "a b\n" {
		t.Errorf("folded = %q, want %q", fold.Str, "a b\n")
	}
	strip := parseOne(t, "x: |-\n  a\n  b\n").Items[0].Val
	if strip.Str != "a\nb" {
		t.Errorf("strip = %q, want %q", strip.Str, "a\nb")
	}
}

func TestSplitKeyValue(t *testing.T) {
	cases := []struct {
		in    string
		key   string
		val   string
		isMap bool
	}{
		{"name: value", "name", "value", true},
		{"empty:", "empty", "", true},
		{"just a scalar", "", "", false},
		{"url: http://x#y", "url", "http://x#y", true},
		{`"quoted key": v`, "quoted key", "v", true},
		{"list: [1, 2]", "list", "[1, 2]", true},
		{"foo # not a key", "", "", false},
	}
	for _, c := range cases {
		k, v, isMap, err := splitKeyValue(c.in)
		if err != nil {
			t.Errorf("splitKeyValue(%q) error: %v", c.in, err)
			continue
		}
		if isMap != c.isMap || k != c.key || v != c.val {
			t.Errorf("splitKeyValue(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.in, k, v, isMap, c.key, c.val, c.isMap)
		}
	}
}

func TestQuotedKeysAreStrings(t *testing.T) {
	v := parseOne(t, `n: "123"`+"\n")
	if v.Items[0].Val.Kind != value.Str || v.Items[0].Val.Str != "123" {
		t.Errorf("quoted 123 should stay a string, got %+v", v.Items[0].Val)
	}
}
