package tui

import (
	"bufio"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ccdisk/internal/term"
)

// Run scans root and runs the interactive UI until the user quits. The
// terminal is restored on every exit path.
func Run(a *App, root string, in, out *os.File) (err error) {
	st, err := term.MakeRaw(in, out)
	if err != nil {
		return err
	}
	defer term.Restore(in, out, st)
	w := bufio.NewWriterSize(out, 256<<10)
	io.WriteString(w, term.EnterUI)
	defer func() {
		io.WriteString(w, term.LeaveUI)
		w.Flush()
	}()

	events := make(chan []term.Event, 16)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := in.Read(buf)
			if n > 0 {
				events <- term.Parse(append([]byte(nil), buf[:n]...))
			}
			if err != nil {
				close(events)
				return
			}
		}
	}()
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGHUP, os.Interrupt)
	defer signal.Stop(sigs)
	resized := term.ResizeNotify()
	tick := time.NewTicker(100 * time.Millisecond) // progress, and resize polling on Windows
	defer tick.Stop()

	a.StartScan(root)
	var canvas *term.Canvas
	var r term.Renderer
	for !a.Quit {
		if cw, ch, err := term.Size(out); err == nil && (canvas == nil || cw != canvas.W || ch != canvas.H) {
			canvas = term.NewCanvas(cw, ch)
			r.Invalidate()
		}
		a.Draw(canvas)
		if _, err := w.Write(r.Frame(canvas)); err != nil {
			return err
		}
		if err := w.Flush(); err != nil {
			return err
		}
		select {
		case evs, ok := <-events:
			if !ok {
				return nil
			}
			for _, ev := range evs {
				a.Handle(ev)
			}
		case f := <-a.Done:
			f()
		case <-sigs:
			a.quit()
		case <-resized:
		case <-tick.C:
		}
	}
	return nil
}
