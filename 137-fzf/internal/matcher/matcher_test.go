package matcher

import (
	"strings"
	"sync/atomic"
	"testing"

	"ccfzf/internal/algo"
	"ccfzf/internal/cli"
	"ccfzf/internal/item"
	"ccfzf/internal/pattern"
)

func fill(lines ...string) *Store {
	s := NewStore()
	b := &item.Builder{Delim: item.DefaultDelimiter()}
	for i, line := range lines {
		s.Append(*b.Build(int32(i), line))
	}
	s.SetDone()
	return s
}

func readStore(t *testing.T, input string, nul bool) *Store {
	t.Helper()
	s := NewStore()
	b := &item.Builder{Delim: item.DefaultDelimiter()}
	if err := Read(strings.NewReader(input), nul, b, s, nil); err != nil {
		t.Fatal(err)
	}
	return s
}

func texts(ms []Match) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = m.Item.Display()
	}
	return out
}

func query(t *testing.T, s *Store, q string, o Options) []Match {
	t.Helper()
	p := pattern.Build(q, pattern.DefaultOptions())
	ms, ok := Run(s, p, 0, s.Count(), o, nil)
	if !ok {
		t.Fatal("search reported cancellation")
	}
	return ms
}

func defaults() Options {
	return Options{Tiebreak: []cli.Criterion{cli.ByLength}, Sort: true}
}

func equal(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestReadLines(t *testing.T) {
	s := readStore(t, "alpha\nbeta\ngamma\n", false)
	if s.Count() != 3 {
		t.Fatalf("count %d", s.Count())
	}
	if s.At(2).Display() != "gamma" {
		t.Errorf("got %q", s.At(2).Display())
	}
	if !s.Done() {
		t.Error("store should be marked done at EOF")
	}
}

func TestReadWithoutTrailingNewline(t *testing.T) {
	if s := readStore(t, "a\nb", false); s.Count() != 2 {
		t.Errorf("count %d", s.Count())
	}
}

func TestReadKeepsEmptyLines(t *testing.T) {
	if s := readStore(t, "a\n\nb\n", false); s.Count() != 3 {
		t.Errorf("count %d", s.Count())
	}
}

func TestReadStripsCarriageReturns(t *testing.T) {
	s := readStore(t, "a\r\nb\r\n", false)
	if s.At(0).Display() != "a" {
		t.Errorf("got %q", s.At(0).Display())
	}
}

func TestReadNulDelimited(t *testing.T) {
	s := readStore(t, "a b\x00c\nd\x00", true)
	if s.Count() != 2 {
		t.Fatalf("count %d", s.Count())
	}
	if s.At(1).Display() != "c\nd" {
		t.Errorf("got %q", s.At(1).Display())
	}
}

func TestStoreSpansChunks(t *testing.T) {
	s := NewStore()
	b := &item.Builder{Delim: item.DefaultDelimiter()}
	const n = chunkSize*2 + 7
	for i := range n {
		s.Append(*b.Build(int32(i), "item"))
	}
	if s.Count() != n {
		t.Fatalf("count %d", s.Count())
	}
	if s.At(n-1).Index != n-1 {
		t.Errorf("last item index %d", s.At(n-1).Index)
	}
}

func TestEmptyQueryReturnsEverythingInOrder(t *testing.T) {
	s := fill("c", "a", "b")
	if got := texts(query(t, s, "", defaults())); !equal(got, "c", "a", "b") {
		t.Errorf("got %q", got)
	}
}

func TestRankingPrefersBetterMatches(t *testing.T) {
	s := fill("a_b_c", "abc", "xxabcxx", "a-b-c")
	got := texts(query(t, s, "abc", defaults()))
	if got[0] != "abc" {
		t.Errorf("best match should come first, got %q", got)
	}
	if len(got) != 4 {
		t.Errorf("all items match fuzzily: %q", got)
	}
}

func TestTiebreakLength(t *testing.T) {
	s := fill("main_longer.go", "main.go")
	got := texts(query(t, s, "main", defaults()))
	if !equal(got, "main.go", "main_longer.go") {
		t.Errorf("got %q", got)
	}
}

func TestTiebreakIndexKeepsInputOrder(t *testing.T) {
	s := fill("bbb", "aaa")
	o := defaults()
	o.Tiebreak = []cli.Criterion{cli.ByIndex}
	if got := texts(query(t, s, "", o)); !equal(got, "bbb", "aaa") {
		t.Errorf("got %q", got)
	}
}

func TestTiebreakBegin(t *testing.T) {
	s := fill("xxxxq", "qxxxx")
	o := defaults()
	o.Tiebreak = []cli.Criterion{cli.ByBegin}
	got := texts(query(t, s, "q", o))
	if got[0] != "qxxxx" {
		t.Errorf("earliest match should win, got %q", got)
	}
}

func TestNoSortKeepsInputOrder(t *testing.T) {
	s := fill("zzzabc", "abc", "a-b-c")
	o := defaults()
	o.Sort = false
	if got := texts(query(t, s, "abc", o)); !equal(got, "zzzabc", "abc", "a-b-c") {
		t.Errorf("got %q", got)
	}
}

func TestTacReversesOrder(t *testing.T) {
	s := fill("one", "two", "three")
	o := defaults()
	o.Sort = false
	o.Tac = true
	if got := texts(query(t, s, "", o)); !equal(got, "three", "two", "one") {
		t.Errorf("got %q", got)
	}
}

func TestTacFlipsTheFinalTiebreak(t *testing.T) {
	s := fill("aaa", "bbb")
	o := defaults()
	o.Tac = true
	if got := texts(query(t, s, "", o)); !equal(got, "bbb", "aaa") {
		t.Errorf("got %q", got)
	}
}

func TestExtendedQueryFiltersAcrossChunks(t *testing.T) {
	s := NewStore()
	b := &item.Builder{Delim: item.DefaultDelimiter()}
	var idx int32
	for i := range chunkSize * 3 {
		text := "file.rs"
		if i%1000 == 0 {
			text = "file.go"
		}
		s.Append(*b.Build(idx, text))
		idx++
	}
	got := query(t, s, "go$", defaults())
	if len(got) != 13 {
		t.Errorf("got %d matches, want 13", len(got))
	}
}

func TestCancellationStopsTheSearch(t *testing.T) {
	s := NewStore()
	b := &item.Builder{Delim: item.DefaultDelimiter()}
	for i := range 200000 {
		s.Append(*b.Build(int32(i), "some/path/to/a/file.go"))
	}
	var cancel atomic.Bool
	cancel.Store(true)
	p := pattern.Build("file", pattern.DefaultOptions())
	if _, ok := Run(s, p, 0, s.Count(), defaults(), &cancel); ok {
		t.Error("a cancelled search should report that it did not finish")
	}
}

func TestMergeSorted(t *testing.T) {
	s := fill("abc", "abcd", "abcde")
	o := defaults()
	all := query(t, s, "abc", o)
	merged := MergeSorted(all[:1], all[1:], o)
	if !equal(texts(merged), texts(all)...) {
		t.Errorf("merge changed the order: %q vs %q", texts(merged), texts(all))
	}
}

func TestIncrementalMatchingMatchesAFullSearch(t *testing.T) {
	// What the interface does while input is still streaming in: match the
	// items that arrived since last time, then merge.
	s := fill("alpha.go", "beta.go", "gamma.rs", "delta.go")
	o := defaults()
	p := pattern.Build("go", pattern.DefaultOptions())
	first, _ := Run(s, p, 0, 2, o, nil)
	second, _ := Run(s, p, 2, 4, o, nil)
	merged := MergeSorted(first, second, o)
	full := query(t, s, "go", o)
	if !equal(texts(merged), texts(full)...) {
		t.Errorf("incremental %q != full %q", texts(merged), texts(full))
	}
}

func TestPositionsMapBackToTheDisplayText(t *testing.T) {
	nth, _ := item.ParseNth("2")
	b := &item.Builder{Delim: item.DefaultDelimiter(), Nth: nth}
	it := b.Build(0, "alpha beta")
	p := pattern.Build("bt", pattern.DefaultOptions())
	pos := Positions(p, it, algo.NewSlab())
	if len(pos) != 2 || pos[0] != 6 || pos[1] != 8 {
		t.Errorf("got %v, want [6 8]", pos)
	}
}

func BenchmarkRunOverManyItems(b *testing.B) {
	s := NewStore()
	builder := &item.Builder{Delim: item.DefaultDelimiter()}
	for i := range 100000 {
		s.Append(*builder.Build(int32(i), "src/internal/pkg/file_controller_test.go"))
	}
	p := pattern.Build("ctrl", pattern.DefaultOptions())
	o := defaults()
	b.ResetTimer()
	for b.Loop() {
		Run(s, p, 0, s.Count(), o, nil)
	}
}
