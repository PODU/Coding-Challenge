package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
)

type config struct {
	number         bool // -n: number every output line
	numberNonBlank bool // -b: number only non-blank lines; wins over -n
}

const usage = `Usage: ccat [OPTIONS] [FILE...]

Concatenate FILE(s) to standard output.
With no FILE, or when FILE is '-', read standard input.

Options:
  -n          number all output lines
  -b          number non-blank output lines (overrides -n)
  --          treat all following arguments as file names
  -h, --help  show this help and exit
`

var errHelp = errors.New("help requested")

// run is main minus os.Exit. It returns 1 for a bad option or if any file
// could not be read.
func run(args []string, stdin io.Reader, out, errw io.Writer) int {
	cfg, operands, err := parseArgs(args)
	if err != nil {
		if errors.Is(err, errHelp) {
			fmt.Fprint(out, usage)
			return 0
		}
		fmt.Fprintf(errw, "ccat: %v\n", err)
		fmt.Fprint(errw, "Try 'ccat --help' for more information.\n")
		return 1
	}

	if len(operands) == 0 {
		operands = []string{"-"}
	}

	return cat(cfg, operands, stdin, out, errw)
}

// parseArgs splits args into options and file operands. Short flags can be
// combined (-nb), and "--" ends option parsing.
func parseArgs(args []string) (config, []string, error) {
	var cfg config
	operands := make([]string, 0, len(args))
	noMoreFlags := false

	for _, a := range args {
		switch {
		case noMoreFlags:
			operands = append(operands, a)
		case a == "--":
			noMoreFlags = true
		case a == "-h" || a == "--help":
			return cfg, nil, errHelp
		case a == "-", a == "", a[0] != '-':
			operands = append(operands, a)
		default:
			for _, c := range a[1:] {
				switch c {
				case 'n':
					cfg.number = true
				case 'b':
					cfg.numberNonBlank = true
				default:
					return cfg, nil, fmt.Errorf("invalid option -- '%c'", c)
				}
			}
		}
	}

	return cfg, operands, nil
}

// cat writes every operand to out. Line numbers carry on from one file to the
// next.
func cat(cfg config, operands []string, stdin io.Reader, out, errw io.Writer) int {
	w := bufio.NewWriter(out)
	defer w.Flush()

	lineNo := 0
	exit := 0

	for _, name := range operands {
		if err := catOne(cfg, name, stdin, w, &lineNo); err != nil {
			// Flush first so the error appears after the output that preceded it.
			w.Flush()
			fmt.Fprintf(errw, "ccat: %s\n", diagnose(name, err))
			exit = 1
		}
	}

	return exit
}

func catOne(cfg config, name string, stdin io.Reader, w *bufio.Writer, lineNo *int) error {
	if name == "-" {
		return processReader(stdin, w, cfg, lineNo)
	}

	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()

	if info, err := f.Stat(); err == nil && info.IsDir() {
		return errIsDir
	}

	return processReader(f, w, cfg, lineNo)
}

var errIsDir = errors.New("is a directory")

// processReader copies r to w, numbering lines if asked. lineNo is shared
// across files. Bytes are passed through unchanged, including a missing final
// newline.
func processReader(r io.Reader, w *bufio.Writer, cfg config, lineNo *int) error {
	if !cfg.number && !cfg.numberNonBlank {
		_, err := io.Copy(w, r)
		return err
	}

	br := bufio.NewReader(r)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			number := cfg.number
			if cfg.numberNonBlank {
				number = line[0] != '\n'
			}
			if number {
				*lineNo++
				fmt.Fprintf(w, "%6d\t", *lineNo)
			}
			if _, werr := w.Write(line); werr != nil {
				return werr
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// diagnose formats err the way cat does: "<file>: <reason>".
func diagnose(name string, err error) string {
	switch {
	case errors.Is(err, errIsDir):
		return fmt.Sprintf("%s: Is a directory", name)
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Sprintf("%s: No such file or directory", name)
	case errors.Is(err, fs.ErrPermission):
		return fmt.Sprintf("%s: Permission denied", name)
	default:
		// Drop the "open <name>:" prefix that *fs.PathError adds.
		var pe *fs.PathError
		if errors.As(err, &pe) {
			return fmt.Sprintf("%s: %v", name, pe.Err)
		}
		return fmt.Sprintf("%s: %v", name, err)
	}
}
