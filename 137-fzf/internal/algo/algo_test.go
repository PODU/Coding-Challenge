package algo

import (
	"strings"
	"testing"
)

func run(fn Fn, caseSensitive bool, text, pattern string) (Result, []int) {
	p := []rune(pattern)
	if !caseSensitive {
		p = []rune(ToLowerString(pattern))
	}
	return fn(caseSensitive, true, NewChars(text), p, true, NewSlab())
}

func assertMatch(t *testing.T, fn Fn, caseSensitive bool, text, pattern string, start, end, score int) {
	t.Helper()
	res, pos := run(fn, caseSensitive, text, pattern)
	if res.Start != start || res.End != end || res.Score != score {
		t.Errorf("%q / %q: got {%d %d %d}, want {%d %d %d}",
			text, pattern, res.Start, res.End, res.Score, start, end, score)
	}
	if res.Matched() && len(pos) != len([]rune(pattern)) {
		t.Errorf("%q / %q: got %d positions, want %d", text, pattern, len(pos), len([]rune(pattern)))
	}
	for _, p := range pos {
		if p < res.Start || p >= res.End {
			t.Errorf("%q / %q: position %d outside [%d,%d)", text, pattern, p, res.Start, res.End)
		}
	}
}

func assertNoMatch(t *testing.T, fn Fn, caseSensitive bool, text, pattern string) {
	t.Helper()
	if res, _ := run(fn, caseSensitive, text, pattern); res.Matched() {
		t.Errorf("%q / %q: unexpected match %+v", text, pattern, res)
	}
}

func TestFuzzyMatchScores(t *testing.T) {
	for _, fn := range []Fn{FuzzyMatchV1, FuzzyMatchV2} {
		assertMatch(t, fn, false, "fooBarbaz1", "oBZ", 2, 9,
			scoreMatch*3+bonusCamel123+scoreGapStart+scoreGapExtension*3)
		assertMatch(t, fn, false, "foo bar baz", "fbb", 0, 9,
			scoreMatch*3+bonusBoundaryWhite*bonusFirstCharMultiplier+bonusBoundaryWhite*2+
				2*scoreGapStart+4*scoreGapExtension)
		assertMatch(t, fn, false, "/AutomatorDocument.icns", "rdoc", 9, 13,
			scoreMatch*4+bonusCamel123+bonusConsecutive*2)
		assertMatch(t, fn, false, "/man1/zshcompctl.1", "zshc", 6, 10,
			scoreMatch*4+bonusBoundaryDelimiter*bonusFirstCharMultiplier+bonusBoundaryDelimiter*3)
	}
}

func TestCaseSensitivity(t *testing.T) {
	for _, fn := range []Fn{FuzzyMatchV1, FuzzyMatchV2} {
		assertNoMatch(t, fn, true, "fooBarbaz", "oBZ")
		if res, _ := run(fn, true, "fooBarbaz", "oBa"); !res.Matched() {
			t.Error("case-sensitive match of oBa in fooBarbaz failed")
		}
	}
}

func TestNoMatch(t *testing.T) {
	for _, fn := range []Fn{FuzzyMatchV1, FuzzyMatchV2} {
		assertNoMatch(t, fn, false, "fooBarbaz", "oBZz")
		assertNoMatch(t, fn, false, "abc", "abcd")
		assertNoMatch(t, fn, false, "", "a")
	}
}

func TestEmptyPatternMatchesEverything(t *testing.T) {
	for _, fn := range []Fn{FuzzyMatchV1, FuzzyMatchV2, ExactMatchNaive, PrefixMatch} {
		if res, _ := run(fn, false, "anything", ""); !res.Matched() {
			t.Error("empty pattern should match")
		}
	}
}

// V2 is allowed to look past the first greedy placement; V1 is not. This is
// the case that motivates having two algorithms at all.
func TestV2BeatsV1WhenTheBetterMatchIsLater(t *testing.T) {
	text, pattern := "axxbxxc abc", "abc"
	v1, _ := run(FuzzyMatchV1, false, text, pattern)
	v2, _ := run(FuzzyMatchV2, false, text, pattern)
	if v1.Start != 0 {
		t.Errorf("v1 should settle for the first placement, got start %d", v1.Start)
	}
	if v2.Start != 8 {
		t.Errorf("v2 should find the word at 8, got start %d", v2.Start)
	}
	if v2.Score <= v1.Score {
		t.Errorf("v2 score %d should beat v1 score %d", v2.Score, v1.Score)
	}
}

func TestConsecutiveAndBoundaryPreferred(t *testing.T) {
	score := func(text string) int {
		res, _ := run(FuzzyMatchV2, false, text, "abc")
		return res.Score
	}
	if score("abc") <= score("axbxc") {
		t.Error("contiguous match should outscore a scattered one")
	}
	if score("foo/abc") <= score("fooabcd") {
		t.Error("match after a delimiter should outscore a mid-word match")
	}
}

func TestPositionsAreAscendingAndCorrect(t *testing.T) {
	_, pos := run(FuzzyMatchV2, false, "foo bar baz", "fbb")
	want := []int{0, 4, 8}
	if len(pos) != 3 {
		t.Fatalf("got %v", pos)
	}
	for i := range want {
		if pos[i] != want[i] {
			t.Fatalf("got %v, want %v", pos, want)
		}
	}
}

func TestExactMatch(t *testing.T) {
	res, pos := run(ExactMatchNaive, false, "src/main/Main.go", "main")
	if res.Start != 4 || res.End != 8 {
		t.Errorf("got %+v, want 4..8", res)
	}
	if len(pos) != 4 {
		t.Errorf("got %d positions", len(pos))
	}
	assertNoMatch(t, ExactMatchNaive, false, "src/main.go", "mian")
}

// Among several occurrences the best-scoring one wins, not the first.
func TestExactMatchPicksBestOccurrence(t *testing.T) {
	res, _ := run(ExactMatchNaive, false, "xmain/main", "main")
	if res.Start != 6 {
		t.Errorf("got start %d, want 6 (the one after the delimiter)", res.Start)
	}
}

func TestPrefixSuffixEqual(t *testing.T) {
	if res, _ := run(PrefixMatch, false, "foobar", "foo"); !res.Matched() || res.Start != 0 {
		t.Errorf("prefix: %+v", res)
	}
	assertNoMatch(t, PrefixMatch, false, "afoobar", "foo")
	if res, _ := run(PrefixMatch, false, "   foobar", "foo"); !res.Matched() || res.Start != 3 {
		t.Errorf("prefix should skip leading whitespace: %+v", res)
	}
	if res, _ := run(SuffixMatch, false, "foobar", "bar"); !res.Matched() || res.End != 6 {
		t.Errorf("suffix: %+v", res)
	}
	assertNoMatch(t, SuffixMatch, false, "foobarx", "bar")
	if res, _ := run(SuffixMatch, false, "foobar  ", "bar"); !res.Matched() {
		t.Error("suffix should ignore trailing whitespace")
	}
	if res, _ := run(EqualMatch, false, "foo", "foo"); !res.Matched() {
		t.Error("equal should match")
	}
	assertNoMatch(t, EqualMatch, false, "foox", "foo")
}

func TestUnicode(t *testing.T) {
	res, pos := run(FuzzyMatchV2, false, "héllo wörld", "hw")
	if !res.Matched() {
		t.Fatal("no match")
	}
	if pos[0] != 0 || pos[1] != 6 {
		t.Errorf("rune positions %v, want [0 6]", pos)
	}
}

func TestSlabIsReusable(t *testing.T) {
	slab := NewSlab()
	texts := []string{"short", "a much longer string than the previous one", "mid length"}
	for range 3 {
		for _, text := range texts {
			FuzzyMatchV2(false, true, NewChars(text), []rune("sn"), true, slab)
		}
	}
	res, _ := FuzzyMatchV2(false, true, NewChars("foo bar baz"), []rune("fbb"), false, slab)
	if res.Score != 78 {
		t.Errorf("score after slab reuse = %d, want 78", res.Score)
	}
}

func TestLongItemDoesNotPanic(t *testing.T) {
	text := strings.Repeat("abcdefghij", 500)
	res, pos := run(FuzzyMatchV2, false, text, "aj")
	if !res.Matched() || len(pos) != 2 {
		t.Errorf("got %+v %v", res, pos)
	}
}

func BenchmarkFuzzyMatchV2(b *testing.B) {
	text := NewChars("src/internal/controller/deployment_controller_test.go")
	pattern := []rune("dctest")
	slab := NewSlab()
	b.ResetTimer()
	for b.Loop() {
		FuzzyMatchV2(false, true, text, pattern, false, slab)
	}
}
