package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Config holds the options that change what run prints.
type Config struct {
	Count     bool // -c
	DupesOnly bool // -d
	UniqOnly  bool // -u
}

// run collapses runs of adjacent identical lines from in and writes the result
// to out. Only the previous line is kept in memory. ReadString is used instead
// of bufio.Scanner so there is no limit on line length.
//
// Lines are compared byte for byte. A last line without a newline is still
// printed, with one added.
func run(cfg Config, in io.Reader, out io.Writer) error {
	r := bufio.NewReader(in)
	w := bufio.NewWriter(out)

	var prev string
	count := 0

	for {
		line, err := r.ReadString('\n')
		if len(line) > 0 || err == nil {
			// Only the '\n' is stripped; a '\r' from CRLF input stays part of
			// the line.
			rec := strings.TrimSuffix(line, "\n")
			if count > 0 && rec == prev {
				count++
			} else {
				if err := emit(w, cfg, prev, count); err != nil {
					return err
				}
				prev = rec
				count = 1
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	if err := emit(w, cfg, prev, count); err != nil {
		return err
	}

	return w.Flush()
}

// emit writes one finished run of count copies of line. count is 0 only for
// empty input.
func emit(w io.Writer, cfg Config, line string, count int) error {
	if count == 0 {
		return nil
	}
	if cfg.DupesOnly && count < 2 {
		return nil
	}
	if cfg.UniqOnly && count != 1 {
		return nil
	}

	var err error
	if cfg.Count {
		// Same layout as GNU uniq -c.
		_, err = fmt.Fprintf(w, "%7d %s\n", count, line)
	} else {
		_, err = fmt.Fprintf(w, "%s\n", line)
	}
	return err
}
