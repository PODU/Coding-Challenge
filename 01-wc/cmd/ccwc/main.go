// Command ccwc is a small clone of the Unix wc utility. It prints line, word,
// character and byte counts in GNU wc's column layout.
package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"ccwc/internal/cli"
	"ccwc/internal/counter"
)

const (
	exitOK      = 0
	exitFileErr = 1 // a file could not be opened or read
	exitUsage   = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

type input struct {
	name    string
	isStdin bool
	label   string // "" means no filename column
}

type result struct {
	counts counter.Counts
	label  string
}

// run is main without the process globals, so tests can drive it.
func run(args []string, stdin io.Reader, out, errOut io.Writer) int {
	opts, err := cli.Parse(args)
	if err != nil {
		if errors.Is(err, cli.ErrHelp) {
			cli.PrintUsage(out)
			return exitOK
		}
		var ue *cli.UsageError
		if errors.As(err, &ue) {
			fmt.Fprintln(errOut, ue.Error())
			fmt.Fprintln(errOut, "Try 'ccwc -h' for more information.")
			return exitUsage
		}
		fmt.Fprintf(errOut, "ccwc: %v\n", err)
		return exitUsage
	}

	opts.ApplyDefaults()

	// Bare stdin gets no label, but an explicit "-" is labeled "-" like GNU wc.
	var inputs []input
	if len(opts.Files) == 0 {
		inputs = append(inputs, input{isStdin: true})
	} else {
		for _, f := range opts.Files {
			if f == "-" {
				inputs = append(inputs, input{isStdin: true, label: "-"})
			} else {
				inputs = append(inputs, input{name: f, label: f})
			}
		}
	}

	exitCode := exitOK
	var results []result
	var total counter.Counts
	allSizesKnown := true

	for _, in := range inputs {
		c, sizeKnown, err := countInput(in, stdin)
		if err != nil {
			fmt.Fprintf(errOut, "ccwc: %s: %s\n", in.label, describeErr(err))
			exitCode = exitFileErr
			allSizesKnown = false
			continue
		}
		if !sizeKnown {
			allSizesKnown = false
		}
		total.Add(c)
		results = append(results, result{counts: c, label: in.label})
	}

	// GNU wc sizes the columns from all results before printing any row.
	width := cli.FieldWidth(cli.SelectedCount(opts), len(results), allSizesKnown, total.Bytes)
	for _, r := range results {
		fmt.Fprintln(out, cli.FormatRow(opts, r.counts, r.label, width))
	}

	if len(opts.Files) > 1 {
		fmt.Fprintln(out, cli.FormatRow(opts, total, "total", width))
	}

	return exitCode
}

// countInput also reports whether the input is a regular file, since that
// decides the column width.
func countInput(in input, stdin io.Reader) (counter.Counts, bool, error) {
	if in.isStdin {
		c, err := counter.Count(stdin)
		return c, false, err
	}

	f, err := os.Open(in.name)
	if err != nil {
		return counter.Counts{}, false, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return counter.Counts{}, false, err
	}
	if info.IsDir() {
		return counter.Counts{}, false, errIsDir
	}

	c, err := counter.Count(f)
	if err != nil {
		return counter.Counts{}, false, err
	}
	return c, info.Mode().IsRegular(), nil
}

var errIsDir = errors.New("is a directory")

// describeErr uses GNU wc's wording instead of the OS error text, which
// differs on Windows.
func describeErr(err error) string {
	switch {
	case errors.Is(err, errIsDir):
		return "Is a directory"
	case errors.Is(err, fs.ErrNotExist):
		return "No such file or directory"
	case errors.Is(err, fs.ErrPermission):
		return "Permission denied"
	default:
		return err.Error()
	}
}
