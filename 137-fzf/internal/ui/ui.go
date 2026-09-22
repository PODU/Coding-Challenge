// Package ui runs the interactive finder: it owns the screen, the key
// bindings, and the search jobs that keep the list up to date while input is
// still streaming in.
package ui

import (
	"strings"
	"sync/atomic"
	"time"

	"ccfzf/internal/algo"
	"ccfzf/internal/cli"
	"ccfzf/internal/item"
	"ccfzf/internal/matcher"
	"ccfzf/internal/pattern"
	"ccfzf/internal/term"
)

// Result is what the interface produced. Exactly one of Aborted or a
// selection is meaningful.
type Result struct {
	Query    string
	Key      string // the --expect key that accepted, if any
	Selected []string
	Aborted  bool
}

// frameInterval is the redraw budget. Keystrokes redraw immediately; this tick
// only exists so that items arriving from the reader and finished search jobs
// reach the screen promptly without redrawing on every single line read.
const frameInterval = 30 * time.Millisecond

type searchResult struct {
	version int
	matches []matcher.Match
	upTo    int
	partial bool // this batch continues an earlier one for the same query
	ok      bool
}

type Terminal struct {
	opts  *cli.Options
	store *matcher.Store
	tty   Screen
	mopts matcher.Options

	// query state
	query  []rune
	cursor int
	pat    *pattern.Pattern

	// search state
	matches     []matcher.Match
	matchedUpTo int
	version     int
	running     bool
	cancel      *atomic.Bool
	results     chan searchResult
	queryDirty  bool

	// list state
	cy      int // index into matches
	offset  int // first visible match
	marks   []*item.Item
	marked  map[int32]bool
	spinner int

	// geometry
	cols, rows int
	origin     int // first screen row the interface may use (1-based)
	altScreen  bool

	preview *previewState
	slab    *algo.Slab
	dirty   bool
}

func New(opts *cli.Options, store *matcher.Store, tty Screen) *Terminal {
	t := &Terminal{
		opts:    opts,
		store:   store,
		tty:     tty,
		mopts:   matcher.Options{Tiebreak: opts.Tiebreak, Sort: opts.Sort, Tac: opts.Tac},
		query:   []rune(opts.Query),
		marked:  map[int32]bool{},
		results: make(chan searchResult, 8),
		slab:    algo.NewSlab(),
	}
	t.cursor = len(t.query)
	t.queryDirty = true
	if opts.Preview != "" {
		t.preview = newPreviewState(opts)
	}
	return t
}

// Run draws the interface and returns once the user accepts or aborts.
func (t *Terminal) Run() (Result, error) {
	if err := t.setup(); err != nil {
		return Result{}, err
	}
	defer t.teardown()

	keys := t.tty.Events()
	ticker := time.NewTicker(frameInterval)
	defer ticker.Stop()

	t.startSearch()
	t.render()

	for {
		select {
		case key, ok := <-keys:
			if !ok {
				return Result{Query: string(t.query), Aborted: true}, nil
			}
			if res, done := t.handleKey(key); done {
				return res, nil
			}
			t.dirty = true
		case r := <-t.results:
			t.applyResult(r)
			t.startSearch()
			t.dirty = true
		case <-ticker.C:
			if !t.store.Done() || t.running {
				t.spinner++
				t.dirty = true
			}
			if t.matchedUpTo < t.store.Count() {
				t.startSearch()
			}
			if t.preview != nil && t.preview.poll() {
				t.dirty = true
			}
			if cols, rows := t.tty.Size(); cols != t.cols || rows != t.rows {
				t.resize()
				t.dirty = true
			}
		}
		if t.dirty {
			t.render()
			t.dirty = false
		}
	}
}

func (t *Terminal) setup() error {
	if err := t.tty.MakeRaw(); err != nil {
		return err
	}
	t.cols, t.rows = t.tty.Size()
	t.altScreen = !t.opts.Height.Set
	if t.altScreen {
		t.origin = 1
		t.tty.EnterAltScreen()
	} else {
		t.reserveHeightArea()
	}
	t.tty.HideCursor()
	t.tty.Flush()
	return nil
}

// reserveHeightArea makes room for a --height window at the bottom of the
// current screen without clearing what is already there: the shell's output
// scrolls up by as much as is needed, and nothing else is touched.
func (t *Terminal) reserveHeightArea() {
	h := t.windowHeight()
	row, _, err := t.tty.CursorPosition()
	if err != nil || row <= 0 {
		row = t.rows - h + 1
	}
	if row+h-1 > t.rows {
		// Not enough room below the cursor: scroll by printing newlines.
		t.tty.Write(strings.Repeat("\n", row+h-1-t.rows))
		t.tty.Flush()
		row = t.rows - h + 1
	}
	t.origin = max(row, 1)
}

func (t *Terminal) windowHeight() int {
	h := t.opts.Height
	if !h.Set {
		return t.rows
	}
	n := h.Size
	if h.Percent {
		n = t.rows * h.Size / 100
	}
	if h.Auto {
		// "~" means "no taller than the content needs".
		need := t.chromeRows() + max(len(t.matches), 1)
		if t.matchedUpTo == 0 && !t.store.Done() {
			need = n
		}
		n = min(n, need)
	}
	return max(min(n, t.rows), t.chromeRows()+1)
}

func (t *Terminal) teardown() {
	if t.preview != nil {
		t.preview.stop()
	}
	if t.altScreen {
		t.tty.ExitAltScreen()
	} else {
		// Wipe the window we borrowed and leave the cursor where it started.
		t.tty.MoveTo(t.origin, 1)
		t.tty.ClearToEndOfScreen()
	}
	t.tty.ShowCursor()
	t.tty.Reset()
	t.tty.Flush()
	t.tty.Restore()
}

func (t *Terminal) resize() {
	t.cols, t.rows = t.tty.Size()
	if !t.altScreen {
		t.origin = max(t.rows-t.windowHeight()+1, 1)
	}
}

func (t *Terminal) startSearch() {
	if t.running {
		return // the result handler restarts the search
	}
	if t.queryDirty {
		t.pat = pattern.Build(string(t.query), t.opts.PatternOptions())
		t.matches = nil
		t.matchedUpTo = 0
		t.queryDirty = false
		t.cy, t.offset = 0, 0
	}
	count := t.store.Count()
	if t.matchedUpTo >= count {
		return
	}
	from, to, version := t.matchedUpTo, count, t.version
	partial := from > 0
	pat := t.pat
	cancel := &atomic.Bool{}
	t.cancel = cancel
	t.running = true
	go func() {
		ms, ok := matcher.Run(t.store, pat, from, to, t.mopts, cancel)
		t.results <- searchResult{version: version, matches: ms, upTo: to, partial: partial, ok: ok}
	}()
}

func (t *Terminal) applyResult(r searchResult) {
	t.running = false
	if r.version != t.version || !r.ok {
		return // the query moved on while this search was running
	}
	if r.partial {
		t.matches = matcher.MergeSorted(t.matches, r.matches, t.mopts.ForPattern(t.pat))
	} else {
		t.matches = r.matches
	}
	t.matchedUpTo = r.upTo
	t.clampCursor()
}

func (t *Terminal) invalidateQuery() {
	t.version++
	t.queryDirty = true
	if t.running && t.cancel != nil {
		t.cancel.Store(true)
	}
	t.startSearch()
}

func (t *Terminal) current() *item.Item {
	if t.cy < 0 || t.cy >= len(t.matches) {
		return nil
	}
	return t.matches[t.cy].Item
}

func (t *Terminal) clampCursor() {
	t.cy = max(min(t.cy, len(t.matches)-1), 0)
	listRows := t.geometry().listRows
	if listRows > 0 {
		t.offset = min(t.offset, t.cy)
		t.offset = max(t.offset, t.cy-listRows+1)
		t.offset = max(min(t.offset, max(len(t.matches)-listRows, 0)), 0)
	}
}

// vmove moves the highlight through the ranking: "down" is always the next,
// worse-scoring match, whichever end of the screen the layout draws it at.
func (t *Terminal) vmove(up bool, n int) {
	if up {
		n = -n
	}
	t.cy += n
	t.clampCursor()
}

func (t *Terminal) toggleMark(it *item.Item) {
	if t.opts.Multi == 0 || it == nil {
		return
	}
	if t.marked[it.Index] {
		delete(t.marked, it.Index)
		for i, m := range t.marks {
			if m.Index == it.Index {
				t.marks = append(t.marks[:i], t.marks[i+1:]...)
				break
			}
		}
		return
	}
	if t.opts.Multi > 0 && len(t.marks) >= t.opts.Multi {
		return
	}
	t.marked[it.Index] = true
	t.marks = append(t.marks, it)
}

func (t *Terminal) selection() []string {
	if len(t.marks) > 0 {
		out := make([]string, len(t.marks))
		for i, m := range t.marks {
			out[i] = m.Raw
		}
		return out
	}
	if it := t.current(); it != nil {
		return []string{it.Raw}
	}
	return nil
}

func (t *Terminal) handleKey(key term.Key) (Result, bool) {
	// --expect turns an ordinary key into an accept that reports which key was
	// used, so a script can act on it.
	if t.opts.Expect[key.Name] {
		return Result{Query: string(t.query), Key: key.Name, Selected: t.selection()}, true
	}

	switch key.Name {
	case "enter":
		return Result{Query: string(t.query), Selected: t.selection()}, true
	case "esc", "ctrl-c", "ctrl-g", "ctrl-q":
		return Result{Query: string(t.query), Aborted: true}, true
	case "ctrl-d":
		if len(t.query) == 0 {
			return Result{Query: "", Aborted: true}, true
		}
		t.deleteChar()
	case "up", "ctrl-k", "ctrl-p":
		t.vmove(true, 1)
	case "down", "ctrl-j", "ctrl-n":
		t.vmove(false, 1)
	case "pgup":
		t.vmove(true, max(t.geometry().listRows-1, 1))
	case "pgdn":
		t.vmove(false, max(t.geometry().listRows-1, 1))
	case "tab":
		if t.opts.Multi != 0 {
			t.toggleMark(t.current())
			t.vmove(false, 1)
		}
	case "shift-tab":
		if t.opts.Multi != 0 {
			t.toggleMark(t.current())
			t.vmove(true, 1)
		}
	case "bspace":
		if t.cursor > 0 {
			t.query = append(t.query[:t.cursor-1], t.query[t.cursor:]...)
			t.cursor--
			t.invalidateQuery()
		}
	case "delete":
		t.deleteChar()
	case "ctrl-w":
		t.deleteWord()
	case "ctrl-u":
		if t.cursor > 0 {
			t.query = t.query[t.cursor:]
			t.cursor = 0
			t.invalidateQuery()
		}
	case "alt-d":
		t.deleteWordForward()
	case "ctrl-a", "home":
		t.cursor = 0
	case "ctrl-e", "end":
		t.cursor = len(t.query)
	case "left", "ctrl-b":
		t.cursor = max(t.cursor-1, 0)
	case "right", "ctrl-f":
		t.cursor = min(t.cursor+1, len(t.query))
	case "alt-b":
		t.cursor = wordStart(t.query, t.cursor)
	case "alt-f":
		t.cursor = wordEnd(t.query, t.cursor)
	case "ctrl-l":
		t.tty.ClearScreen()
	default:
		if key.Printable() {
			t.query = append(t.query[:t.cursor], append([]rune{key.Rune}, t.query[t.cursor:]...)...)
			t.cursor++
			t.invalidateQuery()
		}
	}
	return Result{}, false
}

func (t *Terminal) deleteChar() {
	if t.cursor < len(t.query) {
		t.query = append(t.query[:t.cursor], t.query[t.cursor+1:]...)
		t.invalidateQuery()
	}
}

func (t *Terminal) deleteWord() {
	start := wordStart(t.query, t.cursor)
	if start != t.cursor {
		t.query = append(t.query[:start], t.query[t.cursor:]...)
		t.cursor = start
		t.invalidateQuery()
	}
}

func (t *Terminal) deleteWordForward() {
	end := wordEnd(t.query, t.cursor)
	if end != t.cursor {
		t.query = append(t.query[:t.cursor], t.query[end:]...)
		t.invalidateQuery()
	}
}

func wordStart(q []rune, pos int) int {
	i := pos
	for i > 0 && q[i-1] == ' ' {
		i--
	}
	for i > 0 && q[i-1] != ' ' {
		i--
	}
	return i
}

func wordEnd(q []rune, pos int) int {
	i := pos
	for i < len(q) && q[i] == ' ' {
		i++
	}
	for i < len(q) && q[i] != ' ' {
		i++
	}
	return i
}
