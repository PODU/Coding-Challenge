package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const usage = `Usage: uniq [OPTION]... [INPUT [OUTPUT]]

Filter adjacent matching lines from INPUT (or standard input),
writing to OUTPUT (or standard output).

With no options, matching adjacent lines are collapsed to the first occurrence.

Options:
  -c, --count       prefix lines with the number of occurrences
  -d, --repeated    only print duplicate lines, one for each group
  -u, --unique      only print unique lines
  -h, --help        display this help and exit

Arguments:
  INPUT    file to read; "-" or omitted means standard input
  OUTPUT   file to write; "-" or omitted means standard output
`

type parseResult struct {
	cfg     Config
	inPath  string // "" or "-" => stdin
	outPath string // "" or "-" => stdout
	help    bool
}

// parseArgs parses the arguments after the program name. Options can come
// before or after the file names, short flags can be bundled (-cd), and "--"
// ends option parsing.
func parseArgs(args []string) (parseResult, error) {
	var res parseResult
	var positionals []string
	optsEnded := false

	for _, arg := range args {
		switch {
		case optsEnded:
			positionals = append(positionals, arg)

		case arg == "--":
			optsEnded = true

		case arg == "-" || arg == "":
			positionals = append(positionals, arg)

		case len(arg) > 2 && arg[:2] == "--":
			switch arg {
			case "--count":
				res.cfg.Count = true
			case "--repeated":
				res.cfg.DupesOnly = true
			case "--unique":
				res.cfg.UniqOnly = true
			case "--help":
				res.help = true
			default:
				return res, fmt.Errorf("unrecognized option '%s'", arg)
			}

		case arg[0] == '-':
			for _, c := range arg[1:] {
				switch c {
				case 'c':
					res.cfg.Count = true
				case 'd':
					res.cfg.DupesOnly = true
				case 'u':
					res.cfg.UniqOnly = true
				case 'h':
					res.help = true
				default:
					return res, fmt.Errorf("invalid option -- '%c'", c)
				}
			}

		default:
			positionals = append(positionals, arg)
		}
	}

	if len(positionals) > 2 {
		return res, fmt.Errorf("extra operand '%s'", positionals[2])
	}
	if len(positionals) >= 1 {
		res.inPath = positionals[0]
	}
	if len(positionals) >= 2 {
		res.outPath = positionals[1]
	}
	return res, nil
}

func isStd(path string) bool { return path == "" || path == "-" }

// openInput opens path, or returns stdin with a no-op closer.
func openInput(path string, stdin io.Reader) (io.Reader, func() error, error) {
	if isStd(path) {
		return stdin, func() error { return nil }, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	return f, f.Close, nil
}

// openOutput creates path, or returns stdout with a no-op closer.
func openOutput(path string, stdout io.Writer) (io.Writer, func() error, error) {
	if isStd(path) {
		return stdout, func() error { return nil }, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	return f, f.Close, nil
}

// samePath reports whether a and b name the same file. Output is truncated
// before input is read, so uniq refuses to run when they match.
func samePath(a, b string) bool {
	if isStd(a) || isStd(b) {
		return false
	}
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return a == b
	}
	return absA == absB
}

func main() {
	os.Exit(realMain(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// realMain runs the command and returns its exit code. Tests call it with
// their own streams.
func realMain(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	res, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "uniq: %v\n", err)
		fmt.Fprint(stderr, usage)
		return 1
	}

	if res.help {
		fmt.Fprint(stdout, usage)
		return 0
	}

	if samePath(res.inPath, res.outPath) {
		fmt.Fprintf(stderr, "uniq: refusing to read and write the same file '%s'\n", res.inPath)
		return 1
	}

	in, closeIn, err := openInput(res.inPath, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "uniq: %v\n", err)
		return 1
	}
	defer closeIn()

	out, closeOut, err := openOutput(res.outPath, stdout)
	if err != nil {
		fmt.Fprintf(stderr, "uniq: %v\n", err)
		return 1
	}

	if err := run(res.cfg, in, out); err != nil {
		closeOut()
		fmt.Fprintf(stderr, "uniq: %v\n", err)
		return 1
	}
	if err := closeOut(); err != nil {
		fmt.Fprintf(stderr, "uniq: %v\n", err)
		return 1
	}
	return 0
}
