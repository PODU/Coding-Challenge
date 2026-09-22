package ui

import (
	"strings"
	"testing"

	"ccfzf/internal/cli"
	"ccfzf/internal/item"
	"ccfzf/internal/matcher"
	"ccfzf/internal/term"
)

const (
	screenCols = 60
	screenRows = 20
)

func newApp(t *testing.T, tweak func(*cli.Options), lines ...string) (*fakeScreen, *matcher.Store, chan Result) {
	t.Helper()
	opts := cli.Defaults()
	if tweak != nil {
		tweak(opts)
	}
	store := matcher.NewStore()
	b := opts.ItemBuilder()
	for i, line := range lines {
		store.Append(*b.Build(int32(i), line))
	}
	store.SetDone()

	screen := newFakeScreen(screenCols, screenRows)
	done := make(chan Result, 1)
	go func() {
		res, err := New(opts, store, screen).Run()
		if err != nil {
			t.Error(err)
		}
		done <- res
	}()
	return screen, store, done
}

func key(name string) term.Key { return term.Key{Name: name} }

func TestInitialScreen(t *testing.T) {
	s, _, done := newApp(t, nil, "alpha", "beta", "gamma")
	defer func() { s.send(key("esc")); <-done }()

	s.waitForText(t, "3/3")
	lines := s.lines()
	// Default layout: prompt at the bottom, best match just above the info
	// line, list growing upward.
	if lines[screenRows-1] != ">" {
		t.Errorf("prompt row = %q", lines[screenRows-1])
	}
	if !strings.Contains(lines[screenRows-2], "3/3") {
		t.Errorf("info row = %q", lines[screenRows-2])
	}
	if lines[screenRows-3] != "> alpha" {
		t.Errorf("first match row = %q", lines[screenRows-3])
	}
	if lines[screenRows-4] != "  beta" {
		t.Errorf("second match row = %q", lines[screenRows-4])
	}
}

func TestTypingFilters(t *testing.T) {
	s, _, done := newApp(t, nil, "alpha", "beta", "gamma")
	defer func() { s.send(key("esc")); <-done }()

	typeKeys(s, "mm")
	s.waitForText(t, "1/3")
	s.waitForText(t, "> gamma")
	if strings.Contains(s.text(), "alpha") {
		t.Error("non-matching items should be gone")
	}
	if !strings.Contains(s.text(), "> mm") {
		t.Error("the query should be echoed after the prompt")
	}
}

func TestEnterReturnsTheCurrentItem(t *testing.T) {
	s, _, done := newApp(t, nil, "alpha", "beta", "gamma")
	typeKeys(s, "be")
	s.waitForText(t, "1/3")
	s.send(key("enter"))
	res := <-done
	if len(res.Selected) != 1 || res.Selected[0] != "beta" {
		t.Errorf("got %q", res.Selected)
	}
	if res.Query != "be" {
		t.Errorf("query = %q", res.Query)
	}
	if res.Aborted {
		t.Error("should not be marked aborted")
	}
}

func TestAbortKeys(t *testing.T) {
	for _, name := range []string{"esc", "ctrl-c", "ctrl-g"} {
		s, _, done := newApp(t, nil, "alpha")
		s.waitForText(t, "1/1")
		s.send(key(name))
		res := <-done
		if !res.Aborted || len(res.Selected) != 0 {
			t.Errorf("%s: got %+v", name, res)
		}
	}
}

func TestNavigationDefaultLayout(t *testing.T) {
	s, _, done := newApp(t, nil, "one", "two", "three")
	s.waitForText(t, "3/3")
	// Movement is through the ranking: "down" is the next match, wherever the
	// layout happens to draw it.
	s.send(key("down"))
	s.waitFor(t, "the pointer to move", func(text string) bool {
		return strings.Contains(text, "> two")
	})
	s.send(key("enter"))
	if res := <-done; res.Selected[0] != "two" {
		t.Errorf("got %q", res.Selected)
	}
}

func TestNavigationWrapsAtTheEnds(t *testing.T) {
	s, _, done := newApp(t, nil, "one", "two")
	s.waitForText(t, "2/2")
	// Moving past either end stays there rather than cycling.
	s.send(key("up"), key("up"), key("up"))
	s.send(key("enter"))
	if res := <-done; res.Selected[0] != "one" {
		t.Errorf("got %q", res.Selected)
	}
}

func TestCtrlKeysMoveTheSelection(t *testing.T) {
	s, _, done := newApp(t, nil, "one", "two", "three")
	s.waitForText(t, "3/3")
	s.send(key("ctrl-j"), key("ctrl-j")) // down twice
	s.send(key("ctrl-k"))                // back up once
	s.send(key("enter"))
	if res := <-done; res.Selected[0] != "two" {
		t.Errorf("got %q", res.Selected)
	}
}

func TestReverseLayoutPutsThePromptOnTop(t *testing.T) {
	s, _, done := newApp(t, func(o *cli.Options) { o.Layout = cli.LayoutReverse }, "alpha", "beta")
	defer func() { s.send(key("esc")); <-done }()

	s.waitForText(t, "2/2")
	lines := s.lines()
	if lines[0] != ">" {
		t.Errorf("row 1 = %q", lines[0])
	}
	if !strings.Contains(lines[1], "2/2") {
		t.Errorf("row 2 = %q", lines[1])
	}
	if lines[2] != "> alpha" || lines[3] != "  beta" {
		t.Errorf("list rows = %q / %q", lines[2], lines[3])
	}
}

func TestReverseListKeepsThePromptAtTheBottom(t *testing.T) {
	s, _, done := newApp(t, func(o *cli.Options) { o.Layout = cli.LayoutReverseList }, "alpha", "beta")
	defer func() { s.send(key("esc")); <-done }()

	s.waitForText(t, "2/2")
	lines := s.lines()
	if lines[screenRows-1] != ">" {
		t.Errorf("prompt row = %q", lines[screenRows-1])
	}
	if lines[0] != "> alpha" || lines[1] != "  beta" {
		t.Errorf("list should start at the top: %q / %q", lines[0], lines[1])
	}
}

func TestQueryEditing(t *testing.T) {
	s, _, done := newApp(t, nil, "alpha")
	defer func() { s.send(key("esc")); <-done }()

	typeKeys(s, "abc def")
	s.waitForText(t, "> abc def")

	s.send(key("ctrl-w"))
	s.waitForText(t, "> abc ")

	s.send(key("bspace"))
	s.waitFor(t, "backspace to remove the space", func(text string) bool {
		return strings.Contains(text, "> abc\n") || strings.HasSuffix(strings.TrimRight(text, "\n"), "> abc")
	})

	s.send(key("ctrl-u"))
	s.waitFor(t, "the query to be cleared", func(text string) bool {
		return !strings.Contains(text, "abc")
	})
}

func TestCtrlAAndCtrlEEditInPlace(t *testing.T) {
	s, _, done := newApp(t, nil, "xyz")
	typeKeys(s, "yz")
	s.waitForText(t, "> yz")
	s.send(key("ctrl-a"))
	typeKeys(s, "x")
	s.waitForText(t, "> xyz")
	s.send(key("ctrl-e"))
	typeKeys(s, "q")
	s.waitForText(t, "> xyzq")
	s.send(key("esc"))
	<-done
}

func TestStartingQuery(t *testing.T) {
	s, _, done := newApp(t, func(o *cli.Options) { o.Query = "gam" }, "alpha", "gamma")
	s.waitForText(t, "1/2")
	s.waitForText(t, "> gamma")
	s.send(key("enter"))
	if res := <-done; res.Selected[0] != "gamma" {
		t.Errorf("got %q", res.Selected)
	}
}

func TestMultiSelect(t *testing.T) {
	s, _, done := newApp(t, func(o *cli.Options) { o.Multi = -1 }, "one", "two", "three")
	s.waitForText(t, "3/3")
	s.send(key("tab")) // mark "one", move to "two"
	s.waitForText(t, "(1)")
	s.send(key("tab")) // mark "two"
	s.waitForText(t, "(2)")
	s.send(key("enter"))
	res := <-done
	if len(res.Selected) != 2 || res.Selected[0] != "one" || res.Selected[1] != "two" {
		t.Errorf("got %q", res.Selected)
	}
}

func TestMultiSelectMarkerIsDrawn(t *testing.T) {
	s, _, done := newApp(t, func(o *cli.Options) { o.Multi = -1 }, "one", "two")
	defer func() { s.send(key("esc")); <-done }()
	s.waitForText(t, "2/2")
	s.send(key("tab"))
	s.waitFor(t, "the marker", func(text string) bool {
		return strings.Contains(text, "  >one")
	})
}

func TestShiftTabUnmarks(t *testing.T) {
	s, _, done := newApp(t, func(o *cli.Options) { o.Multi = -1 }, "one", "two")
	s.waitForText(t, "2/2")
	s.send(key("tab")) // mark "one", move on to "two"
	s.waitForText(t, "(1)")
	s.send(key("up"))        // back onto "one"
	s.send(key("shift-tab")) // unmark it
	s.waitFor(t, "the mark to be cleared", func(text string) bool {
		return !strings.Contains(text, "(1)") && !strings.Contains(text, "(2)")
	})
	s.send(key("enter"))
	if res := <-done; len(res.Selected) != 1 {
		t.Errorf("without marks the current item is returned: %q", res.Selected)
	}
}

func TestMultiLimit(t *testing.T) {
	s, _, done := newApp(t, func(o *cli.Options) { o.Multi = 2 }, "one", "two", "three")
	s.waitForText(t, "3/3")
	s.send(key("tab"), key("tab"), key("tab"))
	s.waitForText(t, "(2)")
	s.send(key("enter"))
	if res := <-done; len(res.Selected) != 2 {
		t.Errorf("--multi=2 should cap the selection: %q", res.Selected)
	}
}

func TestTabDoesNothingWithoutMulti(t *testing.T) {
	s, _, done := newApp(t, nil, "one", "two")
	s.waitForText(t, "2/2")
	s.send(key("tab"))
	s.send(key("enter"))
	if res := <-done; len(res.Selected) != 1 || res.Selected[0] != "one" {
		t.Errorf("got %q", res.Selected)
	}
}

func TestExpectKey(t *testing.T) {
	s, _, done := newApp(t, func(o *cli.Options) { o.Expect = map[string]bool{"ctrl-v": true} },
		"alpha", "beta")
	s.waitForText(t, "2/2")
	s.send(key("ctrl-v"))
	res := <-done
	if res.Key != "ctrl-v" {
		t.Errorf("key = %q", res.Key)
	}
	if len(res.Selected) != 1 || res.Selected[0] != "alpha" {
		t.Errorf("selected %q", res.Selected)
	}
}

func TestHeaderLines(t *testing.T) {
	s, _, done := newApp(t, func(o *cli.Options) {
		o.Layout = cli.LayoutReverse
		o.Header = []string{"pick a file", "second line"}
	}, "alpha")
	defer func() { s.send(key("esc")); <-done }()

	s.waitForText(t, "1/1")
	lines := s.lines()
	if lines[2] != "pick a file" || lines[3] != "second line" {
		t.Errorf("header rows = %q / %q", lines[2], lines[3])
	}
	if lines[4] != "> alpha" {
		t.Errorf("list should start below the header: %q", lines[4])
	}
}

func TestInlineInfo(t *testing.T) {
	s, _, done := newApp(t, func(o *cli.Options) { o.Info = cli.InfoInline }, "alpha", "beta")
	defer func() { s.send(key("esc")); <-done }()

	s.waitFor(t, "the counts next to the prompt", func(text string) bool {
		for _, line := range strings.Split(text, "\n") {
			if strings.HasPrefix(line, "> ") && strings.Contains(line, "2/2") {
				return true
			}
		}
		return false
	})
	// With no separate info line the list gets one more row.
	if got := s.lines()[screenRows-2]; got != "> alpha" {
		t.Errorf("row above the prompt = %q", got)
	}
}

func TestHiddenInfo(t *testing.T) {
	s, _, done := newApp(t, func(o *cli.Options) { o.Info = cli.InfoHidden }, "alpha")
	defer func() { s.send(key("esc")); <-done }()

	s.waitForText(t, "> alpha")
	if strings.Contains(s.text(), "1/1") {
		t.Error("--info=hidden should not draw the counts")
	}
}

func TestStreamingItemsAppear(t *testing.T) {
	s, store, done := newApp(t, nil, "first")
	defer func() { s.send(key("esc")); <-done }()

	s.waitForText(t, "1/1")
	b := &item.Builder{Delim: item.DefaultDelimiter()}
	for i := range 5 {
		store.Append(*b.Build(int32(i+1), "later"))
	}
	s.waitForText(t, "6/6")
}

func TestQueryStillFiltersItemsThatArriveLater(t *testing.T) {
	s, store, done := newApp(t, nil, "alpha")
	defer func() { s.send(key("esc")); <-done }()

	typeKeys(s, "beta")
	s.waitForText(t, "0/1")
	b := &item.Builder{Delim: item.DefaultDelimiter()}
	store.Append(*b.Build(1, "beta"))
	s.waitForText(t, "1/2")
	s.waitForText(t, "> beta")
}

func TestLongItemsAreTruncated(t *testing.T) {
	long := strings.Repeat("x", screenCols*2)
	s, _, done := newApp(t, nil, long)
	defer func() { s.send(key("esc")); <-done }()

	s.waitFor(t, "the truncated row", func(text string) bool {
		return strings.Contains(text, "…")
	})
	for _, line := range s.lines() {
		if len([]rune(line)) > screenCols {
			t.Fatalf("line is %d columns wide: %q", len([]rune(line)), line)
		}
	}
}

func TestHeightModeUsesOnlyTheBottomRows(t *testing.T) {
	s, _, done := newApp(t, func(o *cli.Options) {
		o.Height = cli.Height{Set: true, Size: 5}
	}, "alpha", "beta", "gamma")
	defer func() { s.send(key("esc")); <-done }()

	s.waitForText(t, "3/3")
	lines := s.lines()
	for i := range screenRows - 5 {
		if lines[i] != "" {
			t.Errorf("row %d should be untouched, got %q", i+1, lines[i])
		}
	}
	if lines[screenRows-1] != ">" {
		t.Errorf("prompt row = %q", lines[screenRows-1])
	}
}

func TestNoMatchesLeavesAnEmptyList(t *testing.T) {
	s, _, done := newApp(t, nil, "alpha", "beta")
	typeKeys(s, "zzz")
	s.waitForText(t, "0/2")
	s.send(key("enter"))
	if res := <-done; len(res.Selected) != 0 {
		t.Errorf("nothing should be selected, got %q", res.Selected)
	}
}

func TestScrollingKeepsTheCursorVisible(t *testing.T) {
	var lines []string
	for i := range 50 {
		lines = append(lines, "item"+itoa(i))
	}
	s, _, done := newApp(t, nil, lines...)
	s.waitForText(t, "50/50")
	for range 25 {
		s.send(key("down"))
	}
	s.waitFor(t, "the list to scroll", func(text string) bool {
		return strings.Contains(text, "> item25")
	})
	s.send(key("enter"))
	if res := <-done; res.Selected[0] != "item25" {
		t.Errorf("got %q", res.Selected)
	}
}

func TestPageKeys(t *testing.T) {
	var lines []string
	for i := range 50 {
		lines = append(lines, "item"+itoa(i))
	}
	s, _, done := newApp(t, nil, lines...)
	s.waitForText(t, "50/50")
	s.send(key("pgdn"))
	s.waitFor(t, "a page of movement", func(text string) bool {
		return strings.Contains(text, "> item17")
	})
	s.send(key("pgup"))
	s.waitFor(t, "a page back", func(text string) bool {
		return strings.Contains(text, "> item0")
	})
	s.send(key("esc"))
	<-done
}
