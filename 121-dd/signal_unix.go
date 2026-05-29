//go:build unix

package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

// setupSignalHandler prints the current statistics to w on each SIGUSR1. Call
// the returned function to stop.
func setupSignalHandler(s *Stats, w io.Writer) func() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-ch:
				fmt.Fprint(w, s.recordLines()+s.transferLine())
			case <-done:
				return
			}
		}
	}()
	return func() {
		signal.Stop(ch)
		close(done)
	}
}
