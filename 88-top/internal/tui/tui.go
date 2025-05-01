// Package tui draws cctop on the terminal with tcell. The text layout lives in
// render.go, which --once also uses.
package tui

import (
	"context"
	"time"

	"github.com/example/cctop/internal/engine"
	"github.com/example/cctop/internal/model"
	"github.com/example/cctop/internal/source"
	"github.com/gdamore/tcell/v2"
)

type App struct {
	src   source.ProcessSource
	view  model.ViewState
	delay time.Duration
	iters int // 0 = forever

	screen tcell.Screen
	prev   source.Sample
	cur    source.Sample
	snap   model.Snapshot
	frames int
}

func NewApp(src source.ProcessSource, view model.ViewState, delay time.Duration, iters int) *App {
	if delay <= 0 {
		delay = 2 * time.Second
	}
	return &App{src: src, view: view, delay: delay, iters: iters}
}

// Run takes over the terminal until the user quits, ctx is cancelled, or
// iters frames have been shown. The terminal is restored even on a panic.
func (a *App) Run(ctx context.Context) error {
	screen, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	if err := screen.Init(); err != nil {
		return err
	}
	a.screen = screen
	defer screen.Fini() // restores cooked mode + main buffer even on panic

	screen.EnableMouse(0)
	screen.DisableMouse()
	screen.Clear()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	samples := make(chan source.Sample, 1)
	go a.collect(ctx, samples)

	events := make(chan tcell.Event, 16)
	go func() {
		for {
			ev := screen.PollEvent()
			if ev == nil {
				return
			}
			select {
			case events <- ev:
			case <-ctx.Done():
				return
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case s := <-samples:
			a.prev = a.cur
			a.cur = s
			a.recompute()
			a.draw()
			a.frames++
			if a.iters > 0 && a.frames >= a.iters {
				return nil
			}
		case ev := <-events:
			quit := a.handleEvent(ev)
			if quit {
				return nil
			}
		}
	}
}

// collect sends a new sample every interval unless paused.
func (a *App) collect(ctx context.Context, out chan<- source.Sample) {
	// Don't leave the screen blank for the first interval.
	a.sampleOnce(ctx, out)
	t := time.NewTicker(a.delay)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if a.view.Paused {
				continue
			}
			a.sampleOnce(ctx, out)
		}
	}
}

func (a *App) sampleOnce(ctx context.Context, out chan<- source.Sample) {
	s, err := a.src.Sample(ctx)
	if err != nil {
		return
	}
	select {
	case out <- s:
	case <-ctx.Done():
	}
}

func (a *App) recompute() {
	a.snap = engine.Compute(a.prev, a.cur, a.view)
}

// handleEvent applies a key press and reports whether to quit.
func (a *App) handleEvent(ev tcell.Event) bool {
	switch e := ev.(type) {
	case *tcell.EventResize:
		a.screen.Sync()
		a.draw()
	case *tcell.EventKey:
		return a.handleKey(e)
	}
	return false
}

func (a *App) handleKey(e *tcell.EventKey) bool {
	if e.Key() == tcell.KeyCtrlC || e.Key() == tcell.KeyEscape {
		return true
	}
	switch e.Rune() {
	case 'q', 'Q':
		return true
	case 's':
		a.view.Sort = a.view.Sort.Next()
	case 'p':
		a.view.Paused = !a.view.Paused
	case 'P':
		a.view.Sort = model.SortCPU
	case 'M':
		a.view.Sort = model.SortMem
	case 'N':
		a.view.Sort = model.SortPID
	case 'T':
		a.view.Sort = model.SortTime
	case 'r', 'R':
		a.view.Reverse = !a.view.Reverse
	case 'e', 'E':
		a.view.Unit = a.view.Unit.Next()
	case ' ':
		// Force an immediate refresh from the current source.
		if s, err := a.src.Sample(context.Background()); err == nil {
			a.prev = a.cur
			a.cur = s
		}
	default:
		return false
	}
	a.recompute()
	a.draw()
	return false
}

func (a *App) draw() {
	if a.screen == nil {
		return
	}
	a.screen.Clear()
	w, h := a.screen.Size()
	if w <= 0 || h <= 0 {
		a.screen.Show()
		return
	}
	lines := Frame(a.snap, a.delay.Seconds(), w, h)
	last := len(lines) - 1
	for y, line := range lines {
		if y >= h {
			break
		}
		style := tcell.StyleDefault
		switch {
		case y == 5: // column header row
			style = style.Bold(true)
		case y == last: // status bar
			style = style.Reverse(true)
		}
		drawText(a.screen, 0, y, w, style, line)
	}
	a.screen.Show()
}

func drawText(s tcell.Screen, x, y, maxW int, style tcell.Style, text string) {
	col := x
	for _, r := range text {
		if col >= maxW {
			break
		}
		s.SetContent(col, y, r, nil, style)
		col++
	}
}
