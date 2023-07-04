// Package ui has the client's two front-ends: a full-screen tcell UI and a
// line-based one that works without a terminal.
package ui

import "ircclient/internal/client"

// UI shows client events and collects what the user types.
type UI interface {
	// Start must be called before anything else.
	Start() error
	// Lines delivers the user's input, one line at a time.
	Lines() <-chan string
	// Display renders one event. Safe to call from any goroutine.
	Display(ev client.Event)
	SetStatus(nick, channel string)
	// Quit is closed when the user quits from the UI itself, e.g. Ctrl-C.
	Quit() <-chan struct{}
	// Stop restores the terminal.
	Stop()
}
