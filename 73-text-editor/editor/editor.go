// Package editor implements the editor itself: cursor, scrolling, modes,
// search and saving. Render returns what the screen should show, leaving the
// drawing to package term, so all of this can be tested without a terminal.
package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/example/editor/buffer"
	"github.com/example/editor/term"
)

// Mode is the current editing mode.
type Mode int

const (
	ModeControl Mode = iota // normal mode: movement and commands
	ModeInsert
	ModeCommand // typing a ':' command
	ModeSearch  // typing a '/' search
)

// Direction is a cursor movement direction.
type Direction int

const (
	Up Direction = iota
	Down
	Left
	Right
)

// Frame is the screen contents: one entry in Lines per text row, the cursor
// position on screen, and the two bars at the bottom.
type Frame struct {
	Lines     []string
	CursorRow int
	CursorCol int
	Status    string
	Message   string
}

// findState remembers the last query and where the cursor was when the search
// started, so Esc can go back there.
type findState struct {
	query       string
	savedCx     int
	savedCy     int
	savedRowOff int
	savedColOff int
}

// Editor holds all editor state.
type Editor struct {
	buf            *buffer.Buffer
	cx, cy         int // cx is a byte offset into row cy
	rowOff, colOff int // scroll position; colOff is in display columns
	screenRows     int // text rows, i.e. window height minus the two bars
	screenCols     int
	mode           Mode
	cmdline        string
	filename       string
	dirty          int
	status         string
	find           findState
	quit           bool
}

// New creates an editor for a window of rows x cols.
func New(buf *buffer.Buffer, rows, cols int) *Editor {
	e := &Editor{buf: buf}
	e.SetSize(rows, cols)
	return e
}

// SetSize sets the window size. Two rows go to the status and message bars.
func (e *Editor) SetSize(rows, cols int) {
	e.screenRows = max(rows-2, 0)
	e.screenCols = max(cols, 0)
}

// SetFilename sets the file to save to.
func (e *Editor) SetFilename(name string) { e.filename = name }

// Filename returns the file name, or "" if there is none.
func (e *Editor) Filename() string { return e.filename }

// Buffer returns the document being edited.
func (e *Editor) Buffer() *buffer.Buffer { return e.buf }

// Dirty reports whether there are unsaved changes.
func (e *Editor) Dirty() bool { return e.dirty != 0 }

// Mode returns the current mode.
func (e *Editor) Mode() Mode { return e.mode }

// Cursor returns the cursor's row and byte offset within it.
func (e *Editor) Cursor() (cy, cx int) { return e.cy, e.cx }

// SetStatus sets the message line.
func (e *Editor) SetStatus(format string, args ...any) {
	e.status = fmt.Sprintf(format, args...)
}

// currentRow returns nil when the cursor is on the empty line after the end.
func (e *Editor) currentRow() *buffer.Row {
	return e.buf.Row(e.cy)
}

func (e *Editor) rowLen() int {
	if r := e.currentRow(); r != nil {
		return r.Len()
	}
	return 0
}

// MoveCursor moves one step. Left and Right wrap across line ends, and moving
// onto a shorter line pulls the cursor back to its end. The cursor can sit on
// the empty line after the last row, so text can be appended there.
func (e *Editor) MoveCursor(d Direction) {
	switch d {
	case Left:
		if e.cx > 0 {
			e.cx--
		} else if e.cy > 0 {
			e.cy--
			e.cx = e.rowLen()
		}
	case Right:
		row := e.currentRow()
		if row != nil && e.cx < row.Len() {
			e.cx++
		} else if row != nil && e.cx == row.Len() && e.cy < e.buf.NumRows() {
			e.cy++
			e.cx = 0
		}
	case Up:
		if e.cy > 0 {
			e.cy--
		}
	case Down:
		if e.cy < e.buf.NumRows() {
			e.cy++
		}
	}
	if l := e.rowLen(); e.cx > l {
		e.cx = l
	}
	if e.cx < 0 {
		e.cx = 0
	}
}

func (e *Editor) moveToLineStart() { e.cx = 0 }
func (e *Editor) moveToLineEnd()   { e.cx = e.rowLen() }

// InsertChar types ru at the cursor.
func (e *Editor) InsertChar(ru rune) {
	e.buf.InsertChar(e.cy, e.cx, ru)
	e.cx += len(string(ru))
	e.dirty++
}

// InsertNewline is Enter.
func (e *Editor) InsertNewline() {
	e.buf.InsertNewline(e.cy, e.cx)
	e.cy++
	e.cx = 0
	e.dirty++
}

// DeleteChar is Backspace.
func (e *Editor) DeleteChar() {
	if e.cy == 0 && e.cx == 0 {
		return
	}
	if e.cy >= e.buf.NumRows() {
		// Nothing to delete on the line after the end; just step back.
		e.MoveCursor(Left)
		return
	}
	if e.cx == 0 {
		prevLen := e.buf.Row(e.cy - 1).Len()
		e.buf.DeleteChar(e.cy, e.cx)
		e.cy--
		e.cx = prevLen
	} else {
		e.buf.DeleteChar(e.cy, e.cx)
		e.cx--
	}
	e.dirty++
}

// DeleteCharForward deletes the character under the cursor, like 'x'.
func (e *Editor) DeleteCharForward() {
	row := e.currentRow()
	if row == nil {
		return
	}
	if e.cx < row.Len() {
		e.buf.DeleteChar(e.cy, e.cx+1)
		e.dirty++
	} else if e.cy < e.buf.NumRows()-1 {
		// At the end of a line this joins the next line on.
		e.buf.DeleteChar(e.cy+1, 0)
		e.dirty++
	}
}

// scroll keeps the cursor on screen.
func (e *Editor) scroll() {
	rx := 0
	if row := e.currentRow(); row != nil {
		rx = row.CxToRx(e.cx)
	}
	if e.cy < e.rowOff {
		e.rowOff = e.cy
	}
	if e.screenRows > 0 && e.cy >= e.rowOff+e.screenRows {
		e.rowOff = e.cy - e.screenRows + 1
	}
	if rx < e.colOff {
		e.colOff = rx
	}
	if e.screenCols > 0 && rx >= e.colOff+e.screenCols {
		e.colOff = rx - e.screenCols + 1
	}
}

func (e *Editor) pageMove(d Direction) {
	if d == Up {
		e.cy = e.rowOff
		for i := 0; i < e.screenRows; i++ {
			e.MoveCursor(Up)
		}
	} else {
		e.cy = e.rowOff + e.screenRows - 1
		if e.cy > e.buf.NumRows() {
			e.cy = e.buf.NumRows()
		}
		for i := 0; i < e.screenRows; i++ {
			e.MoveCursor(Down)
		}
	}
}

// Find moves the cursor to the next match of query after the cursor, or the
// previous one if dir < 0, wrapping around the document. It reports whether
// anything was found.
func (e *Editor) Find(query string, dir int) bool {
	if query == "" {
		return false
	}
	e.find.query = query
	n := e.buf.NumRows()
	if n == 0 {
		return false
	}
	if dir == 0 {
		dir = 1
	}

	// The rest of the cursor's own row first, then whole rows, ending with the
	// cursor's row again from the other side.
	start := e.cy
	if row := e.buf.Row(start); row != nil {
		from := e.cx + 1
		if dir < 0 {
			from = e.cx - 1
		}
		if idx := indexFrom(row.String(), query, from, dir); idx >= 0 {
			e.cx = idx
			return true
		}
	}
	for i := 1; i <= n; i++ {
		cy := (start + i) % n
		if dir < 0 {
			cy = ((start-i)%n + n) % n
		}
		s := e.buf.Row(cy).String()
		from := 0
		if dir < 0 {
			from = len(s)
		}
		if idx := indexFrom(s, query, from, dir); idx >= 0 {
			e.cy = cy
			e.cx = idx
			return true
		}
	}
	return false
}

// indexFrom returns the first match at or after from (dir > 0) or the last
// match starting at or before from (dir < 0), or -1.
func indexFrom(s, query string, from, dir int) int {
	if dir > 0 {
		if from < 0 {
			from = 0
		}
		if from > len(s) {
			return -1
		}
		i := strings.Index(s[from:], query)
		if i < 0 {
			return -1
		}
		return from + i
	}
	if from > len(s)-len(query) {
		from = len(s) - len(query)
	}
	if from < 0 {
		return -1
	}
	return strings.LastIndex(s[:from+len(query)], query)
}

func (e *Editor) beginSearch() {
	e.find.savedCx = e.cx
	e.find.savedCy = e.cy
	e.find.savedRowOff = e.rowOff
	e.find.savedColOff = e.colOff
}

func (e *Editor) restoreSearch() {
	e.cx = e.find.savedCx
	e.cy = e.find.savedCy
	e.rowOff = e.find.savedRowOff
	e.colOff = e.find.savedColOff
}

// Save writes the buffer to its file and returns the byte count. It writes a
// temp file and renames it over the original, so a failed save never leaves
// the file half-written.
func (e *Editor) Save() (int, error) {
	if e.filename == "" {
		return 0, fmt.Errorf("no filename")
	}
	tmp, err := os.CreateTemp(filepath.Dir(e.filename), ".editor-*.tmp")
	if err != nil {
		return 0, err
	}
	n, err := e.buf.Save(tmp)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), e.filename)
	}
	if err != nil {
		os.Remove(tmp.Name())
		return n, err
	}
	e.dirty = 0
	return n, nil
}

// HandleKey handles one keypress and reports whether the editor should quit.
func (e *Editor) HandleKey(k term.Key) (quit bool) {
	switch e.mode {
	case ModeInsert:
		e.handleInsert(k)
	case ModeCommand:
		e.handleCommand(k)
	case ModeSearch:
		e.handleSearch(k)
	default:
		e.handleControl(k)
	}
	return e.quit
}

func (e *Editor) handleControl(k term.Key) {
	switch k.Type {
	case term.KeyArrowLeft:
		e.MoveCursor(Left)
	case term.KeyArrowRight:
		e.MoveCursor(Right)
	case term.KeyArrowUp:
		e.MoveCursor(Up)
	case term.KeyArrowDown:
		e.MoveCursor(Down)
	case term.KeyHome:
		e.moveToLineStart()
	case term.KeyEnd:
		e.moveToLineEnd()
	case term.KeyPageUp:
		e.pageMove(Up)
	case term.KeyPageDown:
		e.pageMove(Down)
	case term.KeyDelete:
		e.DeleteCharForward()
	case term.KeyCtrl:
		if k.Rune == 'f' {
			e.startSearch()
		}
	case term.KeyRune:
		switch k.Rune {
		case 'h':
			e.MoveCursor(Left)
		case 'l':
			e.MoveCursor(Right)
		case 'k':
			e.MoveCursor(Up)
		case 'j':
			e.MoveCursor(Down)
		case '0':
			e.moveToLineStart()
		case '$':
			e.moveToLineEnd()
		case 'i':
			e.mode = ModeInsert
			e.SetStatus("-- INSERT --")
		case 'a':
			e.MoveCursor(Right)
			e.mode = ModeInsert
			e.SetStatus("-- INSERT --")
		case 'x':
			e.DeleteCharForward()
		case 'n':
			e.findNext(1)
		case 'N':
			e.findNext(-1)
		case ':':
			e.mode = ModeCommand
			e.cmdline = ""
			e.status = ":"
		case '/':
			e.startSearch()
		}
	}
}

func (e *Editor) handleInsert(k term.Key) {
	switch k.Type {
	case term.KeyEsc:
		e.mode = ModeControl
		e.status = ""
		// Like vim, step back onto the last inserted character.
		if e.cx > 0 {
			e.cx--
		}
	case term.KeyEnter:
		e.InsertNewline()
	case term.KeyBackspace:
		e.DeleteChar()
	case term.KeyDelete:
		e.DeleteCharForward()
	case term.KeyArrowLeft:
		e.MoveCursor(Left)
	case term.KeyArrowRight:
		e.MoveCursor(Right)
	case term.KeyArrowUp:
		e.MoveCursor(Up)
	case term.KeyArrowDown:
		e.MoveCursor(Down)
	case term.KeyHome:
		e.moveToLineStart()
	case term.KeyEnd:
		e.moveToLineEnd()
	case term.KeyRune:
		e.InsertChar(k.Rune)
	}
}

func (e *Editor) handleCommand(k term.Key) {
	switch k.Type {
	case term.KeyEsc:
		e.mode = ModeControl
		e.cmdline = ""
		e.status = ""
	case term.KeyEnter:
		e.runCommand(e.cmdline)
		if e.mode == ModeCommand {
			e.mode = ModeControl
		}
		e.cmdline = ""
	case term.KeyBackspace:
		if len(e.cmdline) > 0 {
			e.cmdline = e.cmdline[:len(e.cmdline)-1]
		} else {
			e.mode = ModeControl
		}
		e.status = ":" + e.cmdline
	case term.KeyRune:
		e.cmdline += string(k.Rune)
		e.status = ":" + e.cmdline
	}
}

func (e *Editor) runCommand(cmd string) {
	switch cmd {
	case "w":
		e.doSave()
	case "wq", "x":
		if e.doSave() {
			e.quit = true
		}
	case "q":
		if e.dirty != 0 {
			e.SetStatus("Unsaved changes! Use :q! to discard or :w to save.")
		} else {
			e.quit = true
		}
	case "q!":
		e.quit = true
	default:
		e.SetStatus("Unknown command: %s", cmd)
	}
}

// doSave saves and reports the outcome on the message line.
func (e *Editor) doSave() bool {
	if e.filename == "" {
		e.SetStatus("No filename. (save-as prompt not bound)")
		return false
	}
	n, err := e.Save()
	if err != nil {
		e.SetStatus("Save failed: %v", err)
		return false
	}
	e.SetStatus("%q: %d bytes written", e.filename, n)
	return true
}

func (e *Editor) startSearch() {
	e.beginSearch()
	e.mode = ModeSearch
	e.cmdline = ""
	e.status = "/"
}

func (e *Editor) handleSearch(k term.Key) {
	switch k.Type {
	case term.KeyEsc:
		e.restoreSearch()
		e.mode = ModeControl
		e.cmdline = ""
		e.status = ""
	case term.KeyEnter:
		if e.cmdline != "" {
			if !e.Find(e.cmdline, 1) {
				e.SetStatus("Not found: %s", e.cmdline)
			} else {
				e.status = ""
			}
		}
		e.mode = ModeControl
	case term.KeyBackspace:
		if len(e.cmdline) > 0 {
			e.cmdline = e.cmdline[:len(e.cmdline)-1]
		}
		e.status = "/" + e.cmdline
		e.incrementalFind()
	case term.KeyRune:
		e.cmdline += string(k.Rune)
		e.status = "/" + e.cmdline
		e.incrementalFind()
	}
}

// incrementalFind searches as the query is typed. Each search starts from
// where the cursor was before the search began, not from the previous match.
func (e *Editor) incrementalFind() {
	if e.cmdline == "" {
		e.restoreSearch()
		return
	}
	e.cx = e.find.savedCx
	e.cy = e.find.savedCy
	e.Find(e.cmdline, 1)
}

// findNext implements n and N.
func (e *Editor) findNext(dir int) {
	if e.find.query == "" {
		e.SetStatus("No previous search")
		return
	}
	if !e.Find(e.find.query, dir) {
		e.SetStatus("Not found: %s", e.find.query)
	}
}

// Render scrolls if needed and returns what the screen should show. Rows past
// the end of the file are drawn as "~".
func (e *Editor) Render() Frame {
	e.scroll()
	f := Frame{Lines: make([]string, e.screenRows)}
	for y := 0; y < e.screenRows; y++ {
		fileRow := e.rowOff + y
		if fileRow >= e.buf.NumRows() {
			if e.buf.NumRows() == 0 && y == e.screenRows/3 {
				f.Lines[y] = e.welcomeLine()
			} else {
				f.Lines[y] = "~"
			}
			continue
		}
		render := e.buf.Row(fileRow).Render()
		if e.colOff < len(render) {
			render = render[e.colOff:]
		} else {
			render = ""
		}
		if len(render) > e.screenCols {
			render = render[:e.screenCols]
		}
		f.Lines[y] = render
	}

	rx := 0
	if row := e.currentRow(); row != nil {
		rx = row.CxToRx(e.cx)
	}
	f.CursorRow = e.cy - e.rowOff
	f.CursorCol = rx - e.colOff

	f.Status = e.statusBar()
	f.Message = e.status
	return f
}

func (e *Editor) welcomeLine() string {
	if e.screenCols == 0 {
		return ""
	}
	msg := "Go text editor -- CC#73"
	if len(msg) > e.screenCols {
		msg = msg[:e.screenCols]
	}
	pad := (e.screenCols - len(msg)) / 2
	return "~" + strings.Repeat(" ", pad) + msg
}

// statusBar shows the file name and state on the left and the mode and cursor
// position on the right, fitted to the window width.
func (e *Editor) statusBar() string {
	name := e.filename
	if name == "" {
		name = "[No Name]"
	}
	dirty := ""
	if e.dirty != 0 {
		dirty = " [+]"
	}
	left := fmt.Sprintf("%s%s - %d lines", name, dirty, e.buf.NumRows())
	right := fmt.Sprintf("%s | %d:%d", modeName(e.mode), e.cy+1, e.cx+1)

	width := e.screenCols
	if width <= 0 {
		return left
	}
	if len(left)+len(right) >= width {
		keep := width - len(right) - 1
		if keep < 0 {
			if len(left) > width {
				return left[:width]
			}
			return left
		}
		left = left[:keep]
	}
	gap := width - len(left) - len(right)
	return left + strings.Repeat(" ", gap) + right
}

func modeName(m Mode) string {
	switch m {
	case ModeInsert:
		return "INSERT"
	case ModeCommand:
		return "COMMAND"
	case ModeSearch:
		return "SEARCH"
	default:
		return "CONTROL"
	}
}

// ShouldQuit reports whether a quit has been requested.
func (e *Editor) ShouldQuit() bool { return e.quit }
