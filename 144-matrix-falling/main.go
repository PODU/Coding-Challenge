// Command ccmatrix fills the terminal with falling characters, like the
// digital rain in The Matrix. Coding Challenge #144.
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	os.Exit(realMain(os.Args[1:]))
}

func realMain(args []string) int {
	o, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ccmatrix: %v\n", err)
		var ce unknownColourError
		if !errors.As(err, &ce) {
			fmt.Fprint(os.Stderr, "Try 'ccmatrix -h' for more information.\n")
		}
		return 1
	}
	if o.Help {
		fmt.Print(usage)
		return 0
	}

	st, err := makeRaw(os.Stdin, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ccmatrix: standard input and output must be a terminal: %v\n", err)
		return 1
	}
	// Deferred so the terminal is restored on every path, panics included.
	defer restore(os.Stdin, os.Stdout, st)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()

	s := &ttyScreen{in: os.Stdin, out: os.Stdout, w: bufio.NewWriterSize(os.Stdout, 64<<10)}
	if err := run(ctx, o, s, resizeNotify(), rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))); err != nil {
		restore(os.Stdin, os.Stdout, st)
		fmt.Fprintf(os.Stderr, "ccmatrix: %v\n", err)
		return 1
	}
	return 0
}

type ttyScreen struct {
	in, out *os.File
	w       *bufio.Writer
}

func (t *ttyScreen) Read(p []byte) (int, error)  { return t.in.Read(p) }
func (t *ttyScreen) Write(p []byte) (int, error) { return t.w.Write(p) }
func (t *ttyScreen) Flush() error                { return t.w.Flush() }
func (t *ttyScreen) Size() (int, int, error)     { return termSize(t.out) }
