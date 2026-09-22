package ui

import (
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"ccfzf/internal/term"
)

// fakeScreen is a very small terminal emulator: it understands the handful of
// escape sequences the renderer emits and keeps a character grid that tests
// can assert against. Colours are discarded - what matters for the tests is
// where things land.
type fakeScreen struct {
	mu       sync.Mutex
	cols     int
	rows     int
	grid     [][]rune
	row, col int
	pending  []rune // partial escape sequence across Write calls
	keys     chan term.Key
	frames   int
	alt      bool
}

func newFakeScreen(cols, rows int) *fakeScreen {
	f := &fakeScreen{cols: cols, rows: rows, keys: make(chan term.Key, 64)}
	f.clear()
	f.row, f.col = 1, 1
	return f
}

func (f *fakeScreen) clear() {
	f.grid = make([][]rune, f.rows)
	for i := range f.grid {
		f.grid[i] = []rune(strings.Repeat(" ", f.cols))
	}
}

func (f *fakeScreen) MakeRaw() error                    { return nil }
func (f *fakeScreen) Restore()                          {}
func (f *fakeScreen) Size() (int, int)                  { return f.cols, f.rows }
func (f *fakeScreen) Events() <-chan term.Key           { return f.keys }
func (f *fakeScreen) CursorPosition() (int, int, error) { return f.rows, 1, nil }
func (f *fakeScreen) HideCursor()                       {}
func (f *fakeScreen) ShowCursor()                       {}
func (f *fakeScreen) Reset()                            {}
func (f *fakeScreen) EnterAltScreen()                   { f.mu.Lock(); f.alt = true; f.mu.Unlock() }
func (f *fakeScreen) ExitAltScreen()                    { f.mu.Lock(); f.alt = false; f.mu.Unlock() }
func (f *fakeScreen) MoveTo(row, col int)               { f.Write("\x1b[" + itoa(row) + ";" + itoa(col) + "H") }
func (f *fakeScreen) ClearScreen()                      { f.Write("\x1b[2J\x1b[H") }
func (f *fakeScreen) ClearToEndOfScreen()               { f.Write("\x1b[J") }

func itoa(n int) string { return strconv.Itoa(n) }

func (f *fakeScreen) Flush() {
	f.mu.Lock()
	f.frames++
	f.mu.Unlock()
}

func (f *fakeScreen) Write(s string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	runes := append(f.pending, []rune(s)...)
	f.pending = nil
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r != '\x1b' {
			f.put(r)
			continue
		}
		seq, n, ok := readEscape(runes[i:])
		if !ok {
			f.pending = runes[i:]
			return
		}
		f.apply(seq)
		i += n - 1
	}
}

// readEscape returns the escape sequence starting at runes[0].
func readEscape(runes []rune) (string, int, bool) {
	if len(runes) < 2 {
		return "", 0, false
	}
	if runes[1] != '[' {
		return string(runes[:2]), 2, true
	}
	for i := 2; i < len(runes); i++ {
		if (runes[i] >= '@' && runes[i] <= '~') && runes[i] != '[' {
			return string(runes[:i+1]), i + 1, true
		}
	}
	return "", 0, false
}

func (f *fakeScreen) apply(seq string) {
	if !strings.HasPrefix(seq, "\x1b[") {
		return
	}
	body := seq[2 : len(seq)-1]
	switch seq[len(seq)-1] {
	case 'H':
		row, col := 1, 1
		if parts := strings.SplitN(body, ";", 2); len(parts) == 2 {
			row, _ = strconv.Atoi(parts[0])
			col, _ = strconv.Atoi(parts[1])
		}
		f.row, f.col = max(row, 1), max(col, 1)
	case 'J':
		// Clear from the cursor to the end of the screen.
		for r := f.row; r <= f.rows; r++ {
			start := 1
			if r == f.row {
				start = f.col
			}
			for c := start; c <= f.cols; c++ {
				f.grid[r-1][c-1] = ' '
			}
		}
		if body == "2" {
			f.clear()
		}
	case 'K':
		for c := 1; c <= f.cols; c++ {
			f.grid[f.row-1][c-1] = ' '
		}
	}
}

func (f *fakeScreen) put(r rune) {
	if r == '\n' {
		f.row = min(f.row+1, f.rows)
		return
	}
	if f.row >= 1 && f.row <= f.rows && f.col >= 1 && f.col <= f.cols {
		f.grid[f.row-1][f.col-1] = r
	}
	f.col++
}

func (f *fakeScreen) lines() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, f.rows)
	for i, row := range f.grid {
		out[i] = strings.TrimRight(string(row), " ")
	}
	return out
}

func (f *fakeScreen) text() string { return strings.Join(f.lines(), "\n") }

func (f *fakeScreen) send(keys ...term.Key) {
	for _, k := range keys {
		f.keys <- k
	}
}

func typeKeys(f *fakeScreen, s string) {
	for _, r := range s {
		f.keys <- term.Key{Name: string(r), Rune: r}
	}
}

// waitFor polls the screen until it satisfies the predicate. Searching happens
// on other goroutines, so tests wait for an outcome rather than for a frame.
func (f *fakeScreen) waitFor(t *testing.T, what string, pred func(string) bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if pred(f.text()) {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s; screen was:\n%s", what, f.text())
}

func (f *fakeScreen) waitForText(t *testing.T, want string) {
	t.Helper()
	f.waitFor(t, strconv.Quote(want), func(s string) bool { return strings.Contains(s, want) })
}
