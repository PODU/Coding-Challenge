package pattern

import (
	"testing"

	"ccfzf/internal/algo"
)

func matches(p *Pattern, text string) bool {
	_, _, ok := p.Match(algo.NewChars(text), false, algo.NewSlab())
	return ok
}

func build(query string, tweak func(*Options)) *Pattern {
	opts := DefaultOptions()
	if tweak != nil {
		tweak(&opts)
	}
	return Build(query, opts)
}

func TestTermParsing(t *testing.T) {
	p := build("^core go$ 'exact !inv fuzzy", nil)
	if len(p.TermSets) != 5 {
		t.Fatalf("got %d term sets, want 5", len(p.TermSets))
	}
	want := []struct {
		typ  TermType
		inv  bool
		text string
	}{
		{TermPrefix, false, "core"},
		{TermSuffix, false, "go"},
		{TermExact, false, "exact"},
		{TermFuzzy, true, "inv"},
		{TermFuzzy, false, "fuzzy"},
	}
	for i, w := range want {
		got := p.TermSets[i][0]
		if got.Typ != w.typ || got.Inv != w.inv || string(got.Text) != w.text {
			t.Errorf("term %d = {%v %v %q}, want {%v %v %q}",
				i, got.Typ, got.Inv, string(got.Text), w.typ, w.inv, w.text)
		}
	}
}

func TestEqualTerm(t *testing.T) {
	p := build("^main$", nil)
	if p.TermSets[0][0].Typ != TermEqual {
		t.Fatal("^x$ should parse as an equality term")
	}
	if !matches(p, "main") || matches(p, "mainly") {
		t.Error("equality term matched the wrong items")
	}
}

func TestAllTermsMustMatch(t *testing.T) {
	p := build("src go", nil)
	if !matches(p, "src/main.go") {
		t.Error("both terms present, should match")
	}
	if matches(p, "src/main.rs") {
		t.Error("second term missing, should not match")
	}
}

func TestOrTerms(t *testing.T) {
	p := build("core go$ | rs$ | py$", nil)
	if len(p.TermSets) != 2 {
		t.Fatalf("got %d term sets, want 2", len(p.TermSets))
	}
	if len(p.TermSets[1]) != 3 {
		t.Fatalf("or-set has %d terms, want 3", len(p.TermSets[1]))
	}
	for _, ok := range []string{"core/main.go", "core/main.rs", "core/main.py"} {
		if !matches(p, ok) {
			t.Errorf("%q should match", ok)
		}
	}
	for _, bad := range []string{"core/main.c", "other/main.go"} {
		if matches(p, bad) {
			t.Errorf("%q should not match", bad)
		}
	}
}

func TestNegation(t *testing.T) {
	p := build("go !test", nil)
	if !matches(p, "src/main.go") {
		t.Error("non-test go file should match")
	}
	if matches(p, "src/main_test.go") {
		t.Error("test file should be excluded")
	}
}

func TestNegationOnly(t *testing.T) {
	p := build("!test", nil)
	if !matches(p, "main.go") || matches(p, "main_test.go") {
		t.Error("lone negation misbehaved")
	}
}

func TestExactPrefix(t *testing.T) {
	p := build("'main", nil)
	if !matches(p, "src/main.go") {
		t.Error("exact term should match a contiguous run")
	}
	if matches(p, "src/mian.go") {
		t.Error("exact term should not match out of order")
	}
}

func TestExactModeInvertsQuote(t *testing.T) {
	p := build("mn 'mian", func(o *Options) { o.Exact = true })
	if p.TermSets[0][0].Typ != TermExact {
		t.Error("in exact mode a bare term should be exact")
	}
	if p.TermSets[1][0].Typ != TermFuzzy {
		t.Error("in exact mode a quoted term should be fuzzy")
	}
	if matches(p, "main.go") {
		t.Error("exact term mn should not match main.go")
	}
	if !matches(p, "mn/mian") {
		t.Error("should match when both terms are satisfied")
	}
}

func TestNonExtendedTreatsQueryAsOneTerm(t *testing.T) {
	p := build("a b", func(o *Options) { o.Extended = false })
	if len(p.TermSets) != 1 {
		t.Fatalf("got %d term sets, want 1", len(p.TermSets))
	}
	if !matches(p, "a b") || !matches(p, "xa bx") {
		t.Error("non-extended query should match fuzzily including the space")
	}
	if matches(p, "ab") {
		t.Error("the space is part of the term, not a separator")
	}
}

func TestEscapedSpace(t *testing.T) {
	p := build("foo\\ bar", nil)
	if len(p.TermSets) != 1 {
		t.Fatalf("got %d term sets, want 1", len(p.TermSets))
	}
	if !matches(p, "a foo bar b") {
		t.Error("escaped space should be part of the term")
	}
}

func TestSmartCase(t *testing.T) {
	lower := build("main", nil)
	if !matches(lower, "src/Main.go") {
		t.Error("all-lowercase query should be case-insensitive")
	}
	upper := build("Main", nil)
	if matches(upper, "src/main.go") {
		t.Error("query with an uppercase letter should be case-sensitive")
	}
	if !matches(upper, "src/Main.go") {
		t.Error("case-sensitive query should still match the exact case")
	}
	forced := build("Main", func(o *Options) { o.Case = CaseIgnore })
	if !matches(forced, "src/main.go") {
		t.Error("-i should override smart case")
	}
	respect := build("main", func(o *Options) { o.Case = CaseRespect })
	if matches(respect, "src/Main.go") {
		t.Error("+i should force case sensitivity")
	}
}

func TestEmptyQueryMatchesEverything(t *testing.T) {
	p := build("   ", nil)
	if !p.IsEmpty() {
		t.Error("whitespace-only query should be empty")
	}
	if !matches(p, "anything at all") {
		t.Error("empty query should match")
	}
}

func TestIncompleteModifierIsIgnored(t *testing.T) {
	// The user is mid-keystroke: "^" alone must not reject everything.
	for _, q := range []string{"^", "!", "'", "$"} {
		if p := build(q, nil); !matches(p, "anything") {
			t.Errorf("query %q should match everything while incomplete", q)
		}
	}
}

func TestScoreIsSummedAcrossTerms(t *testing.T) {
	p := build("src go", nil)
	res, _, ok := p.Match(algo.NewChars("src/main.go"), false, algo.NewSlab())
	if !ok {
		t.Fatal("expected a match")
	}
	single := build("src", nil)
	one, _, _ := single.Match(algo.NewChars("src/main.go"), false, algo.NewSlab())
	if res.Score <= one.Score {
		t.Errorf("two terms scored %d, one term scored %d", res.Score, one.Score)
	}
}

func TestMatchOffsetSpansAllTerms(t *testing.T) {
	p := build("src go", nil)
	res, _, _ := p.Match(algo.NewChars("src/main.go"), false, algo.NewSlab())
	if res.Start != 0 || res.End != 11 {
		t.Errorf("got [%d,%d), want [0,11)", res.Start, res.End)
	}
}

func TestPositionsCoverEveryTerm(t *testing.T) {
	p := build("sr go", nil)
	_, pos, _ := p.Match(algo.NewChars("src/main.go"), true, algo.NewSlab())
	if len(pos) != 4 {
		t.Errorf("got %d positions %v, want 4", len(pos), pos)
	}
}

func TestNegationContributesNoPositions(t *testing.T) {
	p := build("!zzz", nil)
	res, pos, ok := p.Match(algo.NewChars("abc"), true, algo.NewSlab())
	if !ok {
		t.Fatal("expected a match")
	}
	if len(pos) != 0 || res.Score != 0 {
		t.Errorf("negation should not highlight or score: %v %+v", pos, res)
	}
}
