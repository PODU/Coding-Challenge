package ui

import (
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"

	"ircclient/internal/client"
)

// line is a single rendered scrollback entry with its colour.
type line struct {
	ts    string
	text  string
	style tcell.Style
}

// Tcell is the full-screen TUI: a scrolling message pane, a status bar, and a
// bottom input line with cursor movement and command history.
type Tcell struct {
	screen tcell.Screen

	mu         sync.Mutex
	buffer     []line
	scroll     int // number of lines scrolled up from the bottom
	input      []rune
	cursor     int
	history    []string
	histPos    int
	statusNick string
	statusChan string

	lines  chan string
	quit   chan struct{}
	redraw chan struct{}
	done   chan struct{}
	once   sync.Once
}

// NewTcell constructs a Tcell UI (the screen is created in Start).
func NewTcell() *Tcell {
	return &Tcell{
		lines:   make(chan string),
		quit:    make(chan struct{}),
		redraw:  make(chan struct{}, 1),
		done:    make(chan struct{}),
		histPos: -1,
	}
}

// Start initialises the tcell screen and launches the event loop.
func (t *Tcell) Start() error {
	s, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	if err := s.Init(); err != nil {
		return err
	}
	s.SetStyle(tcell.StyleDefault)
	s.Clear()
	t.screen = s
	go t.loop()
	t.requestRedraw()
	return nil
}

func (t *Tcell) Lines() <-chan string  { return t.lines }
func (t *Tcell) Quit() <-chan struct{} { return t.quit }

// Display appends an event to the scrollback and requests a redraw.
func (t *Tcell) Display(ev client.Event) {
	if ev.Kind == client.KindShutdown {
		return
	}
	t.mu.Lock()
	t.buffer = append(t.buffer, line{
		ts:    time.Now().Format("15:04:05"),
		text:  ev.Text,
		style: styleFor(ev.Kind),
	})
	if t.scroll > 0 {
		t.scroll++ // keep the viewport anchored while scrolled up
	}
	t.mu.Unlock()
	t.requestRedraw()
}

// SetStatus updates the status bar contents.
func (t *Tcell) SetStatus(nick, channel string) {
	t.mu.Lock()
	t.statusNick = nick
	t.statusChan = channel
	t.mu.Unlock()
	t.requestRedraw()
}

// Stop closes the screen and ends the loop.
func (t *Tcell) Stop() {
	t.signalQuit()
	<-t.done
	if t.screen != nil {
		t.screen.Fini()
	}
}

func (t *Tcell) signalQuit() {
	t.once.Do(func() { close(t.quit) })
}

func (t *Tcell) requestRedraw() {
	select {
	case t.redraw <- struct{}{}:
	default:
	}
}

// loop drives screen events and redraws on its own goroutine.
func (t *Tcell) loop() {
	defer close(t.done)
	evCh := make(chan tcell.Event, 16)
	go func() {
		for {
			ev := t.screen.PollEvent()
			if ev == nil {
				return
			}
			evCh <- ev
		}
	}()

	for {
		select {
		case <-t.quit:
			return
		case <-t.redraw:
			t.draw()
		case ev := <-evCh:
			if t.handleEvent(ev) {
				return
			}
			t.draw()
		}
	}
}

// handleEvent processes one tcell event and returns true if the loop must end.
func (t *Tcell) handleEvent(ev tcell.Event) bool {
	switch ev := ev.(type) {
	case *tcell.EventResize:
		t.screen.Sync()
	case *tcell.EventKey:
		switch ev.Key() {
		case tcell.KeyCtrlC:
			t.signalQuit()
			return true
		case tcell.KeyEnter:
			t.submit()
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			t.backspace()
		case tcell.KeyDelete:
			t.deleteForward()
		case tcell.KeyLeft:
			if t.cursor > 0 {
				t.cursor--
			}
		case tcell.KeyRight:
			if t.cursor < len(t.input) {
				t.cursor++
			}
		case tcell.KeyHome, tcell.KeyCtrlA:
			t.cursor = 0
		case tcell.KeyEnd, tcell.KeyCtrlE:
			t.cursor = len(t.input)
		case tcell.KeyUp:
			t.historyPrev()
		case tcell.KeyDown:
			t.historyNext()
		case tcell.KeyPgUp:
			t.scrollBy(10)
		case tcell.KeyPgDn:
			t.scrollBy(-10)
		case tcell.KeyCtrlU:
			t.input = nil
			t.cursor = 0
		case tcell.KeyRune:
			t.insert(ev.Rune())
		}
	}
	return false
}

func (t *Tcell) submit() {
	t.mu.Lock()
	s := strings.TrimRight(string(t.input), " ")
	t.input = nil
	t.cursor = 0
	if s != "" {
		t.history = append(t.history, s)
	}
	t.histPos = -1
	t.mu.Unlock()
	if s == "" {
		return
	}
	select {
	case t.lines <- s:
	case <-t.quit:
	}
}

func (t *Tcell) insert(r rune) {
	t.mu.Lock()
	t.input = append(t.input, 0)
	copy(t.input[t.cursor+1:], t.input[t.cursor:])
	t.input[t.cursor] = r
	t.cursor++
	t.mu.Unlock()
}

func (t *Tcell) backspace() {
	t.mu.Lock()
	if t.cursor > 0 {
		t.input = append(t.input[:t.cursor-1], t.input[t.cursor:]...)
		t.cursor--
	}
	t.mu.Unlock()
}

func (t *Tcell) deleteForward() {
	t.mu.Lock()
	if t.cursor < len(t.input) {
		t.input = append(t.input[:t.cursor], t.input[t.cursor+1:]...)
	}
	t.mu.Unlock()
}

func (t *Tcell) historyPrev() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.history) == 0 {
		return
	}
	if t.histPos == -1 {
		t.histPos = len(t.history)
	}
	if t.histPos > 0 {
		t.histPos--
	}
	t.input = []rune(t.history[t.histPos])
	t.cursor = len(t.input)
}

func (t *Tcell) historyNext() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.histPos == -1 {
		return
	}
	t.histPos++
	if t.histPos >= len(t.history) {
		t.histPos = -1
		t.input = nil
		t.cursor = 0
		return
	}
	t.input = []rune(t.history[t.histPos])
	t.cursor = len(t.input)
}

func (t *Tcell) scrollBy(n int) {
	t.mu.Lock()
	t.scroll += n
	if t.scroll < 0 {
		t.scroll = 0
	}
	t.mu.Unlock()
}

// draw renders the whole screen under the lock.
func (t *Tcell) draw() {
	if t.screen == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	w, h := t.screen.Size()
	if w <= 0 || h <= 0 {
		return
	}
	t.screen.Clear()

	paneHeight := h - 2 // status bar + input line
	if paneHeight < 1 {
		paneHeight = 1
	}

	// Flatten the buffer into wrapped display rows.
	var rows []struct {
		text  string
		style tcell.Style
	}
	for _, ln := range t.buffer {
		full := ln.ts + " " + ln.text
		for _, seg := range wrap(full, w) {
			rows = append(rows, struct {
				text  string
				style tcell.Style
			}{seg, ln.style})
		}
	}

	// Clamp scroll and pick the visible window.
	maxScroll := len(rows) - paneHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if t.scroll > maxScroll {
		t.scroll = maxScroll
	}
	end := len(rows) - t.scroll
	start := end - paneHeight
	if start < 0 {
		start = 0
	}
	y := 0
	for i := start; i < end && y < paneHeight; i++ {
		drawText(t.screen, 0, y, rows[i].style, rows[i].text)
		y++
	}

	// Status bar.
	ch := t.statusChan
	if ch == "" {
		ch = "(no channel)"
	}
	status := " [" + t.statusNick + "] " + ch
	if t.scroll > 0 {
		status += "  -- scrolled --"
	}
	statusStyle := tcell.StyleDefault.Background(tcell.ColorBlue).Foreground(tcell.ColorWhite)
	drawBar(t.screen, h-2, w, statusStyle, status)

	// Input line.
	prompt := "> "
	drawText(t.screen, 0, h-1, tcell.StyleDefault, prompt+string(t.input))
	t.screen.ShowCursor(len(prompt)+t.cursor, h-1)

	t.screen.Show()
}

func styleFor(k client.Kind) tcell.Style {
	base := tcell.StyleDefault
	switch k {
	case client.KindError:
		return base.Foreground(tcell.ColorRed)
	case client.KindJoin:
		return base.Foreground(tcell.ColorGreen)
	case client.KindPart, client.KindQuitMsg:
		return base.Foreground(tcell.ColorYellow)
	case client.KindNick:
		return base.Foreground(tcell.ColorTeal)
	case client.KindPM:
		return base.Foreground(tcell.ColorFuchsia)
	case client.KindAction:
		return base.Foreground(tcell.ColorPurple)
	case client.KindSelf:
		return base.Foreground(tcell.ColorAqua)
	case client.KindNotice:
		return base.Foreground(tcell.ColorGray)
	case client.KindInfo, client.KindRaw:
		return base.Foreground(tcell.ColorSilver)
	default:
		return base
	}
}

func drawText(s tcell.Screen, x, y int, style tcell.Style, text string) {
	for _, r := range text {
		s.SetContent(x, y, r, nil, style)
		x++
	}
}

func drawBar(s tcell.Screen, y, w int, style tcell.Style, text string) {
	runes := []rune(text)
	for x := 0; x < w; x++ {
		r := ' '
		if x < len(runes) {
			r = runes[x]
		}
		s.SetContent(x, y, r, nil, style)
	}
}

// wrap splits a string into chunks no wider than w columns (rune count).
func wrap(s string, w int) []string {
	if w <= 0 {
		return []string{s}
	}
	runes := []rune(s)
	if len(runes) <= w {
		return []string{s}
	}
	var out []string
	for len(runes) > w {
		out = append(out, string(runes[:w]))
		runes = runes[w:]
	}
	out = append(out, string(runes))
	return out
}

var _ UI = (*Tcell)(nil)
