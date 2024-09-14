package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/example/editor/buffer"
	"github.com/example/editor/term"
)

func load(s string) *buffer.Buffer {
	b, err := buffer.Load(strings.NewReader(s))
	if err != nil {
		panic(err)
	}
	return b
}

// newEd builds an editor with a window of rows x cols (full height incl. bars).
func newEd(s string, rows, cols int) *Editor {
	return New(load(s), rows, cols)
}

func typeRunes(e *Editor, s string) {
	for _, r := range s {
		e.HandleKey(term.RuneKey(r))
	}
}

func TestMoveClampTopBottomLeftRight(t *testing.T) {
	e := newEd("abc\ndef\nghi", 24, 80)
	// Up at top stays at row 0.
	e.MoveCursor(Up)
	if cy, _ := e.Cursor(); cy != 0 {
		t.Fatalf("Up at top: cy = %d, want 0", cy)
	}
	// Left at (0,0) stays.
	e.MoveCursor(Left)
	if cy, cx := e.Cursor(); cy != 0 || cx != 0 {
		t.Fatalf("Left at origin: (%d,%d), want (0,0)", cy, cx)
	}
	// Move to end of doc and past bottom.
	for i := 0; i < 10; i++ {
		e.MoveCursor(Down)
	}
	if cy, _ := e.Cursor(); cy != e.Buffer().NumRows() {
		t.Fatalf("Down past bottom: cy = %d, want %d (virtual EOF line)", cy, e.Buffer().NumRows())
	}
}

func TestMoveDownOntoShorterLineClampsCx(t *testing.T) {
	e := newEd("hello\nhi", 24, 80)
	e.MoveCursor(Right)
	e.MoveCursor(Right)
	e.MoveCursor(Right)
	e.MoveCursor(Right) // cx = 4 on "hello"
	e.MoveCursor(Down)  // onto "hi" (len 2)
	if cy, cx := e.Cursor(); cy != 1 || cx != 2 {
		t.Fatalf("clamp onto shorter line: (%d,%d), want (1,2)", cy, cx)
	}
}

func TestLeftWrapAndRightWrap(t *testing.T) {
	e := newEd("ab\ncd", 24, 80)
	e.MoveCursor(Down) // (1,0)
	e.MoveCursor(Left) // wrap to end of "ab" -> (0,2)
	if cy, cx := e.Cursor(); cy != 0 || cx != 2 {
		t.Fatalf("left-wrap: (%d,%d), want (0,2)", cy, cx)
	}
	e.MoveCursor(Right) // wrap forward to start of next line -> (1,0)
	if cy, cx := e.Cursor(); cy != 1 || cx != 0 {
		t.Fatalf("right-wrap: (%d,%d), want (1,0)", cy, cx)
	}
}

func TestHomeEnd(t *testing.T) {
	e := newEd("hello", 24, 80)
	e.moveToLineEnd()
	if _, cx := e.Cursor(); cx != 5 {
		t.Fatalf("End: cx = %d, want 5", cx)
	}
	e.moveToLineStart()
	if _, cx := e.Cursor(); cx != 0 {
		t.Fatalf("Home: cx = %d, want 0", cx)
	}
}

func TestVerticalScrollOffsetTracksCursor(t *testing.T) {
	// 20 rows, window height 5 -> screenRows = 3.
	var sb strings.Builder
	for i := 0; i < 20; i++ {
		sb.WriteString("line\n")
	}
	e := newEd(sb.String(), 5, 80)
	if e.screenRows != 3 {
		t.Fatalf("screenRows = %d, want 3", e.screenRows)
	}
	// Move cursor to row 10.
	for i := 0; i < 10; i++ {
		e.MoveCursor(Down)
	}
	e.Render() // triggers scroll()
	if e.rowOff != 10-3+1 {
		t.Fatalf("rowOff = %d, want %d", e.rowOff, 10-3+1)
	}
	// Cursor should be on the last visible text row.
	f := e.Render()
	if f.CursorRow != e.screenRows-1 {
		t.Fatalf("CursorRow = %d, want %d", f.CursorRow, e.screenRows-1)
	}
}

func TestHorizontalScrollOffset(t *testing.T) {
	long := strings.Repeat("x", 100)
	e := newEd(long, 5, 10) // screenCols = 10
	for i := 0; i < 50; i++ {
		e.MoveCursor(Right)
	}
	f := e.Render()
	if e.colOff != 50-10+1 {
		t.Fatalf("colOff = %d, want %d", e.colOff, 50-10+1)
	}
	if f.CursorCol != e.screenCols-1 {
		t.Fatalf("CursorCol = %d, want %d", f.CursorCol, e.screenCols-1)
	}
	// The visible slice should be 10 columns wide.
	if len(f.Lines[0]) != 10 {
		t.Fatalf("visible width = %d, want 10", len(f.Lines[0]))
	}
}

func TestModeToggleInsertEscape(t *testing.T) {
	e := newEd("abc", 24, 80)
	e.HandleKey(term.RuneKey('i'))
	if e.Mode() != ModeInsert {
		t.Fatalf("after i: mode = %v, want Insert", e.Mode())
	}
	e.HandleKey(term.Key{Type: term.KeyEsc})
	if e.Mode() != ModeControl {
		t.Fatalf("after Esc: mode = %v, want Control", e.Mode())
	}
}

func TestCommandModeAccumulates(t *testing.T) {
	e := newEd("abc", 24, 80)
	e.HandleKey(term.RuneKey(':'))
	if e.Mode() != ModeCommand {
		t.Fatalf("after ':': mode = %v, want Command", e.Mode())
	}
	e.HandleKey(term.RuneKey('w'))
	e.HandleKey(term.RuneKey('q'))
	if e.cmdline != "wq" {
		t.Fatalf("cmdline = %q, want wq", e.cmdline)
	}
}

func TestEditSequenceThroughHandleKey(t *testing.T) {
	e := newEd("", 24, 80)
	e.HandleKey(term.RuneKey('i')) // insert mode
	typeRunes(e, "hello")
	e.HandleKey(term.Key{Type: term.KeyEnter})
	typeRunes(e, "world")
	e.HandleKey(term.Key{Type: term.KeyBackspace}) // delete 'd'
	if e.Buffer().NumRows() != 2 {
		t.Fatalf("NumRows = %d, want 2", e.Buffer().NumRows())
	}
	if e.Buffer().Row(0).String() != "hello" || e.Buffer().Row(1).String() != "worl" {
		t.Fatalf("rows = %q, %q", e.Buffer().Row(0).String(), e.Buffer().Row(1).String())
	}
	if !e.Dirty() {
		t.Fatal("expected dirty after edits")
	}
}

func TestControlXDeletesUnderCursor(t *testing.T) {
	e := newEd("abc", 24, 80)
	e.HandleKey(term.RuneKey('x')) // delete 'a'
	if e.Buffer().Row(0).String() != "bc" {
		t.Fatalf("got %q, want bc", e.Buffer().Row(0).String())
	}
}

func TestBackspaceJoinsLines(t *testing.T) {
	e := newEd("foo\nbar", 24, 80)
	e.MoveCursor(Down) // (1,0)
	e.HandleKey(term.RuneKey('i'))
	e.HandleKey(term.Key{Type: term.KeyBackspace})
	if e.Buffer().NumRows() != 1 || e.Buffer().Row(0).String() != "foobar" {
		t.Fatalf("join failed: %d rows, row0=%q", e.Buffer().NumRows(), e.Buffer().Row(0).String())
	}
	if cy, cx := e.Cursor(); cy != 0 || cx != 3 {
		t.Fatalf("cursor after join = (%d,%d), want (0,3)", cy, cx)
	}
}

func TestFindForwardBackwardAndWrap(t *testing.T) {
	e := newEd("alpha\nbeta\ngamma beta", 24, 80)
	if !e.Find("beta", 1) {
		t.Fatal("forward find failed")
	}
	if cy, cx := e.Cursor(); cy != 1 || cx != 0 {
		t.Fatalf("first match (%d,%d), want (1,0)", cy, cx)
	}
	// Next forward finds the second "beta" on row 2.
	if !e.Find("beta", 1) {
		t.Fatal("forward next failed")
	}
	if cy, cx := e.Cursor(); cy != 2 || cx != 6 {
		t.Fatalf("second match (%d,%d), want (2,6)", cy, cx)
	}
	// Forward again wraps back to row 1.
	if !e.Find("beta", 1) {
		t.Fatal("wrap find failed")
	}
	if cy, _ := e.Cursor(); cy != 1 {
		t.Fatalf("wrap match cy = %d, want 1", cy)
	}
}

func TestFindBackward(t *testing.T) {
	e := newEd("one two\nthree two\nfour", 24, 80)
	// Start at end of doc.
	for i := 0; i < 20; i++ {
		e.MoveCursor(Down)
	}
	if !e.Find("two", -1) {
		t.Fatal("backward find failed")
	}
	if cy, cx := e.Cursor(); cy != 1 || cx != 6 {
		t.Fatalf("backward match (%d,%d), want (1,6)", cy, cx)
	}
}

func TestFindEmptyAndAbsent(t *testing.T) {
	e := newEd("abc", 24, 80)
	if e.Find("", 1) {
		t.Fatal("empty query should be no-op")
	}
	if e.Find("zzz", 1) {
		t.Fatal("absent query should return false")
	}
	if cy, cx := e.Cursor(); cy != 0 || cx != 0 {
		t.Fatalf("cursor moved on failed search: (%d,%d)", cy, cx)
	}
}

func TestSearchCancelRestoresCursor(t *testing.T) {
	e := newEd("alpha\nbeta\ngamma", 24, 80)
	e.HandleKey(term.RuneKey('/'))
	typeRunes(e, "gamma") // incremental find jumps cursor to row 2
	if cy, _ := e.Cursor(); cy != 2 {
		t.Fatalf("incremental cursor cy = %d, want 2", cy)
	}
	e.HandleKey(term.Key{Type: term.KeyEsc}) // cancel
	if cy, cx := e.Cursor(); cy != 0 || cx != 0 {
		t.Fatalf("cursor not restored on cancel: (%d,%d), want (0,0)", cy, cx)
	}
	if e.Mode() != ModeControl {
		t.Fatalf("mode after cancel = %v, want Control", e.Mode())
	}
}

func TestSearchEnterRuns(t *testing.T) {
	e := newEd("alpha\nbeta\ngamma", 24, 80)
	e.HandleKey(term.RuneKey('/'))
	typeRunes(e, "beta")
	e.HandleKey(term.Key{Type: term.KeyEnter})
	if cy, _ := e.Cursor(); cy != 1 {
		t.Fatalf("search-enter cy = %d, want 1", cy)
	}
	if e.Mode() != ModeControl {
		t.Fatalf("mode = %v, want Control", e.Mode())
	}
}

func TestSaveRoundTripTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	e := newEd("", 24, 80)
	e.SetFilename(path)
	e.HandleKey(term.RuneKey('i'))
	typeRunes(e, "hello world")
	e.HandleKey(term.Key{Type: term.KeyEsc})
	if !e.Dirty() {
		t.Fatal("expected dirty before save")
	}
	n, err := e.Save()
	if err != nil {
		t.Fatal(err)
	}
	if e.Dirty() {
		t.Fatal("expected clean after save")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world\n" {
		t.Fatalf("file content = %q, want %q", string(data), "hello world\n")
	}
	if n != len("hello world\n") {
		t.Fatalf("bytes written = %d, want %d", n, len("hello world\n"))
	}
}

func TestOpenEditSaveViaCommands(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.txt")
	if err := os.WriteFile(path, []byte("first\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := load("first\n")
	e := New(b, 24, 80)
	e.SetFilename(path)
	// Append " line" at end of row 0.
	e.moveToLineEnd()
	e.HandleKey(term.RuneKey('i'))
	typeRunes(e, " line")
	e.HandleKey(term.Key{Type: term.KeyEsc})
	// :wq
	e.HandleKey(term.RuneKey(':'))
	typeRunes(e, "wq")
	quit := e.HandleKey(term.Key{Type: term.KeyEnter})
	if !quit {
		t.Fatal(":wq should request quit")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "first line\n" {
		t.Fatalf("file = %q, want %q", string(data), "first line\n")
	}
}

func TestQuitDirtyGuard(t *testing.T) {
	e := newEd("abc", 24, 80)
	e.HandleKey(term.RuneKey('i'))
	typeRunes(e, "z")
	e.HandleKey(term.Key{Type: term.KeyEsc})
	// :q should be refused while dirty.
	e.HandleKey(term.RuneKey(':'))
	typeRunes(e, "q")
	if quit := e.HandleKey(term.Key{Type: term.KeyEnter}); quit {
		t.Fatal(":q must not quit while dirty")
	}
	// :q! forces quit.
	e.HandleKey(term.RuneKey(':'))
	typeRunes(e, "q!")
	if quit := e.HandleKey(term.Key{Type: term.KeyEnter}); !quit {
		t.Fatal(":q! must quit")
	}
}

func TestRenderSnapshotEmptyBuffer(t *testing.T) {
	e := newEd("", 5, 40) // screenRows = 3
	f := e.Render()
	if len(f.Lines) != 3 {
		t.Fatalf("len(Lines) = %d, want 3", len(f.Lines))
	}
	// Welcome banner sits at row screenRows/3 == 1; others are "~".
	if f.Lines[0] != "~" {
		t.Fatalf("line 0 = %q, want ~", f.Lines[0])
	}
	if !strings.HasPrefix(f.Lines[1], "~") || !strings.Contains(f.Lines[1], "CC#73") {
		t.Fatalf("welcome line = %q", f.Lines[1])
	}
	if !strings.Contains(f.Status, "[No Name]") || !strings.Contains(f.Status, "CONTROL") {
		t.Fatalf("status = %q", f.Status)
	}
	if f.CursorRow != 0 || f.CursorCol != 0 {
		t.Fatalf("cursor = (%d,%d), want (0,0)", f.CursorRow, f.CursorCol)
	}
}

func TestRenderSnapshotText(t *testing.T) {
	e := newEd("hello\nworld", 4, 40) // screenRows = 2
	f := e.Render()
	if f.Lines[0] != "hello" || f.Lines[1] != "world" {
		t.Fatalf("lines = %q", f.Lines)
	}
	if !strings.Contains(f.Status, "2 lines") {
		t.Fatalf("status = %q, want 2 lines", f.Status)
	}
}

func TestRenderTabCursorAlignment(t *testing.T) {
	e := newEd("a\tb", 5, 80)
	// Move cursor onto 'b' (cx = 2). Render column should be 8.
	e.MoveCursor(Right)
	e.MoveCursor(Right)
	f := e.Render()
	if f.CursorCol != 8 {
		t.Fatalf("CursorCol over tab = %d, want 8", f.CursorCol)
	}
}

func TestStatusBarWidth(t *testing.T) {
	e := newEd("abc", 5, 40)
	f := e.Render()
	if len(f.Status) != 40 {
		t.Fatalf("status width = %d, want 40", len(f.Status))
	}
}
