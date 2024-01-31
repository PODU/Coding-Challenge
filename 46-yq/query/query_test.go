package query

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// mustNode decodes a single-document YAML string into its root node.
func mustNode(t *testing.T, src string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) == 1 {
		return doc.Content[0]
	}
	return &doc
}

// scalarValues renders a result stream as a slice of scalar string values.
func scalarValues(t *testing.T, nodes []*yaml.Node) []string {
	t.Helper()
	out := make([]string, len(nodes))
	for i, n := range nodes {
		if n.Kind != yaml.ScalarNode {
			t.Fatalf("expected scalar, got kind %v", n.Kind)
		}
		out[i] = n.Value
	}
	return out
}

// --- Lexer tests ----------------------------------------------------------

func TestLexer(t *testing.T) {
	tests := []struct {
		src  string
		want []tokenType
	}{
		{".", []tokenType{tDot, tEOF}},
		{".a.b", []tokenType{tDot, tIdent, tDot, tIdent, tEOF}},
		{`.["k"]`, []tokenType{tDot, tLBracket, tString, tRBracket, tEOF}},
		{".[0]", []tokenType{tDot, tLBracket, tInt, tRBracket, tEOF}},
		{".[-1]", []tokenType{tDot, tLBracket, tInt, tRBracket, tEOF}},
		{".[]", []tokenType{tDot, tLBracket, tRBracket, tEOF}},
		{".a | .b", []tokenType{tDot, tIdent, tPipe, tDot, tIdent, tEOF}},
		{".a?", []tokenType{tDot, tIdent, tQuestion, tEOF}},
		{"[.a]", []tokenType{tLBracket, tDot, tIdent, tRBracket, tEOF}},
	}
	for _, tt := range tests {
		toks, err := lex(tt.src)
		if err != nil {
			t.Fatalf("lex(%q): %v", tt.src, err)
		}
		if len(toks) != len(tt.want) {
			t.Fatalf("lex(%q): got %d tokens, want %d", tt.src, len(toks), len(tt.want))
		}
		for i, w := range tt.want {
			if toks[i].typ != w {
				t.Errorf("lex(%q) token %d: got %v, want %v", tt.src, i, toks[i].typ, w)
			}
		}
	}
}

func TestLexerErrors(t *testing.T) {
	for _, src := range []string{`."unterminated`, "@", ".-"} {
		if _, err := lex(src); err == nil {
			t.Errorf("lex(%q): expected error, got nil", src)
		}
	}
}

// --- Parser tests ---------------------------------------------------------

func TestParserShapes(t *testing.T) {
	tests := []struct {
		src  string
		want string // expected String() rendering
	}{
		{"", "."},
		{".", "."},
		{".a", ".a"},
		{".a.b.c", ".a | .b | .c"},
		{".a?", ".a?"},
		{`.["k"]`, ".k"},
		{`["k"]`, ".k"},
		{".[0]", ".[0]"},
		{".[-1]", ".[-1]"},
		{".[]", ".[]"},
		{".a[] | .b", ".a | .[] | .b"},
		{"[.a]", "[.a]"},
		{"[.a[].b]", "[.a | .[] | .b]"},
	}
	for _, tt := range tests {
		f, err := Compile(tt.src)
		if err != nil {
			t.Fatalf("Compile(%q): %v", tt.src, err)
		}
		if got := f.String(); got != tt.want {
			t.Errorf("Compile(%q).String() = %q, want %q", tt.src, got, tt.want)
		}
	}
}

func TestParserErrors(t *testing.T) {
	for _, src := range []string{"..", ".[", "|.a", ".a |", "[.a", "]", "?"} {
		if _, err := Compile(src); err == nil {
			t.Errorf("Compile(%q): expected parse error, got nil", src)
		}
	}
}

// --- Evaluator tests ------------------------------------------------------

const evalFixture = `
quotes:
  - id: 1
    quote: first
    author: Alpha
  - id: 2
    quote: second
    author: Beta
meta:
  count: 2
`

func TestEvaluator(t *testing.T) {
	root := mustNode(t, evalFixture)

	tests := []struct {
		filter string
		want   []string // scalar values expected in the stream
	}{
		{".meta.count", []string{"2"}},
		{".quotes[0].author", []string{"Alpha"}},
		{".quotes[1].quote", []string{"second"}},
		{".quotes[1] | .quote", []string{"second"}},
		{".quotes[-1].author", []string{"Beta"}},
		{".quotes[].author", []string{"Alpha", "Beta"}},
		{`.["meta"].count`, []string{"2"}},
		{`["meta"]["count"]`, []string{"2"}},
		{".quotes[99]", []string{"null"}}, // out of range -> null
		{".missing?", nil},                // optional missing -> empty
		{".quotes[0].nope?", nil},         // optional missing key
		{".meta | .[]", []string{"2"}},    // iterate a mapping's values
	}
	for _, tt := range tests {
		f, err := Compile(tt.filter)
		if err != nil {
			t.Fatalf("Compile(%q): %v", tt.filter, err)
		}
		out, err := f.Eval(root)
		if err != nil {
			t.Fatalf("Eval(%q): %v", tt.filter, err)
		}
		got := scalarValues(t, out)
		if !equalStrings(got, tt.want) {
			t.Errorf("Eval(%q) = %v, want %v", tt.filter, got, tt.want)
		}
	}
}

func TestEvaluatorCollect(t *testing.T) {
	root := mustNode(t, evalFixture)
	f, err := Compile("[.quotes[].author]")
	if err != nil {
		t.Fatal(err)
	}
	out, err := f.Eval(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Kind != yaml.SequenceNode {
		t.Fatalf("collect should yield one sequence, got %d nodes", len(out))
	}
	got := scalarValues(t, out[0].Content)
	if !equalStrings(got, []string{"Alpha", "Beta"}) {
		t.Errorf("collected = %v", got)
	}
}

func TestEvaluatorErrors(t *testing.T) {
	root := mustNode(t, evalFixture)
	for _, filter := range []string{
		".missing",         // missing key, no '?'
		".quotes[0].id[0]", // index a scalar
		".meta[0]",         // index a mapping
		".quotes.author",   // field on a sequence
	} {
		f, err := Compile(filter)
		if err != nil {
			t.Fatalf("Compile(%q): %v", filter, err)
		}
		if _, err := f.Eval(root); err == nil {
			t.Errorf("Eval(%q): expected eval error, got nil", filter)
		}
	}
}

func TestOptionalSuppressesTypeError(t *testing.T) {
	root := mustNode(t, evalFixture)
	f, err := Compile(".quotes[0].id[0]?") // index a scalar with '?'
	if err != nil {
		t.Fatal(err)
	}
	out, err := f.Eval(root)
	if err != nil {
		t.Fatalf("optional should suppress error: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected empty stream, got %d", len(out))
	}
}

func TestAliasResolution(t *testing.T) {
	const src = `
defaults: &d
  host: localhost
dev:
  settings: *d
`
	root := mustNode(t, src)
	f, _ := Compile(".dev.settings")
	out, err := f.Eval(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Kind != yaml.MappingNode {
		t.Fatalf("expected resolved mapping, got %v", out)
	}
	// The extracted subtree must be self-contained (no anchor/alias).
	var b strings.Builder
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(out[0]); err != nil {
		t.Fatal(err)
	}
	enc.Close()
	if strings.Contains(b.String(), "*") || strings.Contains(b.String(), "&") {
		t.Errorf("resolved subtree still contains alias/anchor: %q", b.String())
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
