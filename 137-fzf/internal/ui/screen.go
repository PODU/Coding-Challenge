package ui

import "ccfzf/internal/term"

// Screen is everything the interface needs from a terminal. Depending on the
// interface rather than on *term.Terminal is what lets the whole interactive
// loop - layout, key handling, highlighting - be driven by tests against a
// virtual screen.
type Screen interface {
	MakeRaw() error
	Restore()
	Size() (cols, rows int)
	Events() <-chan term.Key
	CursorPosition() (row, col int, err error)

	Write(s string)
	Flush()
	MoveTo(row, col int)
	ClearScreen()
	ClearToEndOfScreen()
	HideCursor()
	ShowCursor()
	EnterAltScreen()
	ExitAltScreen()
	Reset()
}
