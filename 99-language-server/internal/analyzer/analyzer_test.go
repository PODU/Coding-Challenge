package analyzer

import (
	"reflect"
	"strings"
	"testing"

	"greetls/internal/lsp"
)

func rng(sl, sc, el, ec int) lsp.Range {
	return lsp.Range{
		Start: lsp.Position{Line: sl, Character: sc},
		End:   lsp.Position{Line: el, Character: ec},
	}
}

// A document exercising every diagnostic rule produces exactly the
// expected diagnostics with correct ranges and severities (R-D1..R-D5).
func TestDiagnosticsAllRules(t *testing.T) {
	src := strings.Join([]string{
		`# greetings config`,          // 0 comment
		`greet "World" @style=formal`, // 1 ok
		`lang en`,                     // 2 ok (first lang)
		`lang fr`,                     // 3 duplicate lang -> Warning
		`repeat 3`,                    // 4 ok
		`repeat foo`,                  // 5 non-integer -> Error
		`greet`,                       // 6 missing quoted arg -> Error
		`notakeyword here`,            // 7 unknown keyword -> Warning
		`# TODO fix later`,            // 8 TODO marker -> Information
		``,                            // 9 trailing empty line
	}, "\n")

	want := []lsp.Diagnostic{
		{Range: rng(3, 0, 3, 4), Severity: lsp.SeverityWarning, Source: "greetls", Message: "duplicate lang directive"},
		{Range: rng(5, 7, 5, 10), Severity: lsp.SeverityError, Source: "greetls", Message: "repeat value must be a positive integer"},
		{Range: rng(6, 0, 6, 5), Severity: lsp.SeverityError, Source: "greetls", Message: `greet requires a quoted argument, e.g. greet "World"`},
		{Range: rng(7, 0, 7, 11), Severity: lsp.SeverityWarning, Source: "greetls", Message: "unknown keyword: notakeyword"},
		{Range: rng(8, 2, 8, 6), Severity: lsp.SeverityInformation, Source: "greetls", Message: "found TODO marker"},
	}

	got := Diagnostics(src)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diagnostics mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

// A valid document produces zero diagnostics (but a non-nil slice).
func TestDiagnosticsClean(t *testing.T) {
	src := "# clean\ngreet \"Hello\"\nlang en\nrepeat 2\n"
	got := Diagnostics(src)
	if got == nil {
		t.Fatal("Diagnostics returned nil, want empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("want 0 diagnostics, got %+v", got)
	}
}

// repeat with no argument is flagged.
func TestDiagnosticsRepeatMissingArg(t *testing.T) {
	got := Diagnostics("repeat\n")
	if len(got) != 1 || got[0].Severity != lsp.SeverityError {
		t.Fatalf("want one error for bare repeat, got %+v", got)
	}
}

// Hover over a keyword returns its docs; hover over whitespace or out of
// range returns nil.
func TestHover(t *testing.T) {
	text := "repeat 3\n"

	h := Hover(text, lsp.Position{Line: 0, Character: 2}) // inside "repeat"
	if h == nil {
		t.Fatal("hover over 'repeat' returned nil")
	}
	if !strings.Contains(h.Contents.Value, "repeat") {
		t.Errorf("hover doc missing keyword: %q", h.Contents.Value)
	}
	if h.Range == nil || *h.Range != rng(0, 0, 0, 6) {
		t.Errorf("hover range = %+v, want 0,0-0,6", h.Range)
	}

	if Hover(text, lsp.Position{Line: 0, Character: 6}) != nil {
		t.Error("hover over whitespace should be nil")
	}
	if Hover(text, lsp.Position{Line: 0, Character: 7}) != nil {
		t.Error("hover over a number should be nil")
	}
	if Hover(text, lsp.Position{Line: 9, Character: 0}) != nil {
		t.Error("hover on out-of-range line should be nil")
	}
}

// hover over an @attribute resolves the attribute name.
func TestHoverAttribute(t *testing.T) {
	text := `greet "Hi" @style=formal`
	// '@' at byte 11, "style" starts at 12.
	h := Hover(text, lsp.Position{Line: 0, Character: 13})
	if h == nil || !strings.Contains(h.Contents.Value, "style") {
		t.Fatalf("expected style hover, got %+v", h)
	}
}

// Completion at line start returns keywords; after '@' returns attributes.
func TestCompletion(t *testing.T) {
	kw := labels(Completion("", lsp.Position{Line: 0, Character: 0}))
	for _, want := range []string{"greet", "lang", "repeat"} {
		if !kw[want] {
			t.Errorf("keyword completion missing %q (got %v)", want, kw)
		}
	}
	if kw["style"] {
		t.Error("keyword completion should not include attribute 'style'")
	}

	line := `greet "World" @`
	attr := labels(Completion(line, lsp.Position{Line: 0, Character: len(line)}))
	if !attr["style"] {
		t.Errorf("attribute completion missing 'style' (got %v)", attr)
	}
	if attr["greet"] {
		t.Error("attribute completion should not include 'greet'")
	}
}

func labels(items []lsp.CompletionItem) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, it := range items {
		m[it.Label] = true
	}
	return m
}
