package item

import (
	"reflect"
	"testing"
)

func TestAwkTokenize(t *testing.T) {
	got := Tokenize("  aa bb  cc ", DefaultDelimiter())
	want := []string{"aa ", "bb  ", "cc "}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStringDelimiterKeepsSeparator(t *testing.T) {
	got := Tokenize("a:b:c", ParseDelimiter(":"))
	want := []string{"a:", "b:", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRegexDelimiter(t *testing.T) {
	got := Tokenize("a1b22c", ParseDelimiter("[0-9]+"))
	want := []string{"a1", "b22", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseNth(t *testing.T) {
	cases := map[string][]Range{
		"1":         {{1, 1}},
		"2..":       {{2, 0}},
		"..3":       {{0, 3}},
		"2..4":      {{2, 4}},
		"-1":        {{-1, -1}},
		"..":        {{0, 0}},
		"1,3..4,-1": {{1, 1}, {3, 4}, {-1, -1}},
	}
	for spec, want := range cases {
		got, err := ParseNth(spec)
		if err != nil {
			t.Errorf("%q: %v", spec, err)
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %v, want %v", spec, got, want)
		}
	}
	for _, bad := range []string{"0", "x", "1..x", ""} {
		if _, err := ParseNth(bad); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestTransform(t *testing.T) {
	tokens := Tokenize("alpha beta gamma delta", DefaultDelimiter())
	cases := map[string]string{
		"2":    "beta",
		"2..3": "beta gamma",
		"-1":   "delta",
		"..2":  "alpha beta",
		"3..":  "gamma delta",
		"..":   "alpha beta gamma delta",
		"1,-1": "alpha delta",
		"9":    "",
	}
	for spec, want := range cases {
		ranges, err := ParseNth(spec)
		if err != nil {
			t.Fatalf("%q: %v", spec, err)
		}
		if got := Transform(tokens, ranges); got != want {
			t.Errorf("nth=%q: got %q, want %q", spec, got, want)
		}
	}
}

func TestStripAnsi(t *testing.T) {
	text, spans := StripAnsi("\x1b[31mred\x1b[0m plain \x1b[1;32mbold green\x1b[m")
	if text != "red plain bold green" {
		t.Errorf("got %q", text)
	}
	want := []Span{{0, 3, "31"}, {10, 20, "1;32"}}
	if !reflect.DeepEqual(spans, want) {
		t.Errorf("got %v, want %v", spans, want)
	}
}

func TestStripAnsiNoEscapes(t *testing.T) {
	text, spans := StripAnsi("plain text")
	if text != "plain text" || spans != nil {
		t.Errorf("got %q %v", text, spans)
	}
}

func TestStripAnsiIgnoresNonColourSequences(t *testing.T) {
	text, spans := StripAnsi("a\x1b[2Kb")
	if text != "ab" || len(spans) != 0 {
		t.Errorf("got %q %v", text, spans)
	}
}

func TestBuildPlain(t *testing.T) {
	b := &Builder{Delim: DefaultDelimiter()}
	it := b.Build(0, "src/main.go")
	if it.Display() != "src/main.go" || it.Match().String() != "src/main.go" {
		t.Errorf("got %q / %q", it.Display(), it.Match().String())
	}
	if it.MatchOffset() != 0 {
		t.Errorf("offset %d", it.MatchOffset())
	}
}

func TestBuildNthRestrictsMatchingButNotDisplay(t *testing.T) {
	nth, _ := ParseNth("2")
	b := &Builder{Delim: DefaultDelimiter(), Nth: nth}
	it := b.Build(0, "alpha beta gamma")
	if it.Display() != "alpha beta gamma" {
		t.Errorf("display %q", it.Display())
	}
	if it.Match().String() != "beta" {
		t.Errorf("match %q", it.Match().String())
	}
	if it.MatchOffset() != 6 {
		t.Errorf("offset %d, want 6", it.MatchOffset())
	}
}

func TestBuildWithNthChangesDisplay(t *testing.T) {
	withNth, _ := ParseNth("2..")
	b := &Builder{Delim: DefaultDelimiter(), WithNth: withNth}
	it := b.Build(0, "alpha beta gamma")
	if it.Display() != "beta gamma" {
		t.Errorf("display %q", it.Display())
	}
	if it.Raw != "alpha beta gamma" {
		t.Errorf("raw should be untouched, got %q", it.Raw)
	}
}

// A token owns the delimiter that followed it, so reordering fields drops the
// separator that used to sit between them - the same as fzf.
func TestBuildWithNthReorderLosesTheSeparator(t *testing.T) {
	withNth, _ := ParseNth("2,1")
	b := &Builder{Delim: DefaultDelimiter(), WithNth: withNth}
	if got := b.Build(0, "alpha beta").Display(); got != "betaalpha" {
		t.Errorf("display %q", got)
	}
}

func TestBuildAnsi(t *testing.T) {
	b := &Builder{Delim: DefaultDelimiter(), Ansi: true}
	it := b.Build(0, "\x1b[31mred\x1b[0m file")
	if it.Display() != "red file" {
		t.Errorf("display %q", it.Display())
	}
	if it.Raw != "\x1b[31mred\x1b[0m file" {
		t.Error("raw should keep the escapes for output")
	}
	if len(it.Colors) != 1 {
		t.Errorf("colors %v", it.Colors)
	}
}

func TestBuildWithoutAnsiKeepsEscapesVisible(t *testing.T) {
	b := &Builder{Delim: DefaultDelimiter()}
	it := b.Build(0, "\x1b[31mred\x1b[0m")
	if it.Display() != "\x1b[31mred\x1b[0m" {
		t.Error("without --ansi the escapes are ordinary characters")
	}
}
