package ui

import (
	"bufio"
	"fmt"
	"io"
	"sync"
	"time"

	"ircclient/internal/client"
)

// Plain reads input lines from in and writes events to out. It doesn't need a
// terminal, so it works in scripts and tests.
type Plain struct {
	in    io.Reader
	out   io.Writer
	lines chan string
	quit  chan struct{}

	once sync.Once
	mu   sync.Mutex
}

func NewPlain(in io.Reader, out io.Writer) *Plain {
	return &Plain{
		in:    in,
		out:   out,
		lines: make(chan string),
		quit:  make(chan struct{}),
	}
}

func (p *Plain) Start() error {
	go func() {
		sc := bufio.NewScanner(p.in)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			select {
			case p.lines <- sc.Text():
			case <-p.quit:
				return
			}
		}
		// EOF means the user is done.
		p.signalQuit()
	}()
	return nil
}

func (p *Plain) Lines() <-chan string { return p.lines }

// Display writes a timestamped, prefixed line for the event.
func (p *Plain) Display(ev client.Event) {
	if ev.Kind == client.KindShutdown {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintf(p.out, "%s %s\n", time.Now().Format("15:04:05"), ev.Text)
}

func (p *Plain) SetStatus(nick, channel string) {
	if channel == "" {
		channel = "(no channel)"
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintf(p.out, "-- status: %s on %s --\n", nick, channel)
}

func (p *Plain) Quit() <-chan struct{} { return p.quit }

func (p *Plain) Stop() { p.signalQuit() }

func (p *Plain) signalQuit() {
	p.once.Do(func() { close(p.quit) })
}

var _ UI = (*Plain)(nil)
