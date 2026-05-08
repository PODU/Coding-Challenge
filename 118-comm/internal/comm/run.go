package comm

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
)

// errOrderFatal stops the walk under --check-order. The message has already
// been printed by the time it's returned.
var errOrderFatal = errors.New("input is not in sorted order")

// Run runs comm with args and returns the exit status.
func Run(args []string, stdin io.Reader, out, errw io.Writer) int {
	cfg, err := Parse(args)
	if err != nil {
		if errors.Is(err, errHelp) {
			fmt.Fprint(out, usageText)
			return 0
		}
		fmt.Fprintf(errw, "comm: %s\n", err)
		fmt.Fprint(errw, usageText)
		return 1
	}

	r1, r2, closeAll, err := openInputs(cfg, stdin)
	if err != nil {
		fmt.Fprintf(errw, "comm: %s\n", err)
		return 1
	}
	defer closeAll()

	a := newLineSource(r1, 1, cfg.caseInsensitive)
	b := newLineSource(r2, 2, cfg.caseInsensitive)
	if err := a.start(); err != nil {
		fmt.Fprintf(errw, "comm: %s\n", err)
		return 1
	}
	if err := b.start(); err != nil {
		fmt.Fprintf(errw, "comm: %s\n", err)
		return 1
	}

	bw := bufio.NewWriter(out)
	cw := &columnWriter{w: bw, delimiter: cfg.delimiter, suppress: cfg.suppress}

	warned := false
	onOrder := func(fileNo int) error {
		if cfg.noCheckOrder {
			return nil
		}
		if cfg.checkOrder {
			fmt.Fprintf(errw, "comm: file %d is not in sorted order\n", fileNo)
			return errOrderFatal
		}
		// By default, warn once and carry on.
		if !warned {
			fmt.Fprintf(errw, "comm: file %d is not in sorted order\n", fileNo)
			warned = true
		}
		return nil
	}

	c, walkErr := mergeWalk(a, b, cw, cfg.caseInsensitive, onOrder)

	if cfg.total {
		// Printed even when every column is suppressed.
		fmt.Fprintf(bw, "%d%s%d%s%d%stotal\n",
			c.col1, cfg.delimiter, c.col2, cfg.delimiter, c.col3, cfg.delimiter)
	}

	if err := bw.Flush(); err != nil {
		fmt.Fprintf(errw, "comm: %s\n", err)
		return 1
	}

	if walkErr != nil {
		if !errors.Is(walkErr, errOrderFatal) {
			fmt.Fprintf(errw, "comm: %s\n", walkErr)
		}
		return 1
	}
	return 0
}

// openInputs opens both inputs ("-" is stdin) and returns a func that closes
// them.
func openInputs(cfg Config, stdin io.Reader) (r1, r2 io.Reader, closeAll func(), err error) {
	var toClose []io.Closer
	closeAll = func() {
		for _, c := range toClose {
			c.Close()
		}
	}

	open := func(name string) (io.Reader, error) {
		if name == "-" {
			return stdin, nil
		}
		f, err := os.Open(name)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		toClose = append(toClose, f)
		return f, nil
	}

	r1, err = open(cfg.file1)
	if err != nil {
		closeAll()
		return nil, nil, nil, err
	}
	r2, err = open(cfg.file2)
	if err != nil {
		closeAll()
		return nil, nil, nil, err
	}
	return r1, r2, closeAll, nil
}
