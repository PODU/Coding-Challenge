package main

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
)

const version = "cchead 1.0.0"

const usage = `Usage: cchead [OPTION]... [FILE]...

Print the first part of FILEs to standard output. With no FILE, or when
FILE is '-', read standard input.

Options:
  -n, --lines=[-]N    print the first N lines (default 10);
                      with leading '-', print all but the last N lines
  -c, --bytes=[-]N    print the first N bytes;
                      with leading '-', print all but the last N bytes
  -q, --quiet, --silent   never print '==> FILE <==' headers
  -v, --verbose           always print headers
  -h, --help          display this help and exit
      --version       output version information and exit

Examples:
  cchead test.txt                first 10 lines of test.txt
  cchead -n1 test.txt            first line
  cchead -n 5 a.txt b.txt        first 5 lines of each, with headers
  cchead -c 31 test.txt          first 31 bytes
  cchead -n -3 test.txt          all but the last 3 lines
  cat test.txt | cchead -n 2     first 2 lines from stdin
`

type mode int

const (
	modeLines mode = iota
	modeBytes
)

type headerMode int

const (
	headerAuto headerMode = iota // header only when >1 file
	headerNever
	headerAlways
)

type config struct {
	m        mode
	count    int64
	negative bool
	headers  headerMode
}

// usageErr is a bad command line; it exits with status 2.
type usageErr struct{ msg string }

func (e *usageErr) Error() string { return e.msg }

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run does the real work of main and returns the exit status, so tests can
// call it directly.
func run(args []string, stdin io.Reader, out, errOut io.Writer) int {
	cfg, operands, action, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(errOut, "cchead: %s\n", err.Error())
		fmt.Fprint(errOut, "Try 'cchead --help' for more information.\n")
		return 2
	}
	switch action {
	case actionHelp:
		fmt.Fprint(out, usage)
		return 0
	case actionVersion:
		fmt.Fprintln(out, version)
		return 0
	}

	if len(operands) == 0 {
		operands = []string{"-"}
	}

	bw := bufio.NewWriter(out)
	exit := 0
	printHeaders := shouldPrintHeaders(cfg, len(operands))
	first := true

	for _, name := range operands {
		var r io.Reader
		var closer io.Closer
		display := name

		if name == "-" {
			r = stdin
			display = "standard input"
		} else {
			f, ferr := os.Open(name)
			if ferr != nil {
				// Flush first so the warning lands after the output it follows.
				bw.Flush()
				fmt.Fprintf(errOut, "cchead: %s\n", friendlyOpenErr(name, ferr))
				exit = 1
				first = false
				continue
			}
			if info, serr := f.Stat(); serr == nil && info.IsDir() {
				f.Close()
				bw.Flush()
				fmt.Fprintf(errOut, "cchead: %s: Is a directory\n", name)
				exit = 1
				first = false
				continue
			}
			r = f
			closer = f
		}

		if printHeaders {
			if !first {
				fmt.Fprint(bw, "\n")
			}
			fmt.Fprintf(bw, "==> %s <==\n", display)
		}
		first = false

		if cerr := dispatch(bw, r, cfg); cerr != nil {
			bw.Flush()
			fmt.Fprintf(errOut, "cchead: %s: %v\n", display, cerr)
			exit = 1
		}
		if closer != nil {
			closer.Close()
		}
	}

	if ferr := bw.Flush(); ferr != nil && exit == 0 {
		fmt.Fprintf(errOut, "cchead: write error: %v\n", ferr)
		exit = 1
	}
	return exit
}

func dispatch(w io.Writer, r io.Reader, cfg config) error {
	switch {
	case cfg.m == modeLines && !cfg.negative:
		return copyFirstLines(w, r, cfg.count)
	case cfg.m == modeLines && cfg.negative:
		return copyAllButLastLines(w, r, cfg.count)
	case cfg.m == modeBytes && !cfg.negative:
		return copyFirstBytes(w, r, cfg.count)
	default:
		return copyAllButLastBytes(w, r, cfg.count)
	}
}

func shouldPrintHeaders(cfg config, nFiles int) bool {
	switch cfg.headers {
	case headerNever:
		return false
	case headerAlways:
		return true
	default:
		return nFiles > 1
	}
}

type actionKind int

const (
	actionRun actionKind = iota
	actionHelp
	actionVersion
)

// parseArgs handles both short and long flag forms. If both -n and -c are
// given, the last one wins.
func parseArgs(args []string) (config, []string, actionKind, error) {
	cfg := config{m: modeLines, count: 10, headers: headerAuto}
	var operands []string
	noMoreFlags := false

	i := 0
	for i < len(args) {
		arg := args[i]

		if noMoreFlags || arg == "-" || arg == "" || !strings.HasPrefix(arg, "-") {
			operands = append(operands, arg)
			i++
			continue
		}

		if arg == "--" {
			noMoreFlags = true
			i++
			continue
		}

		if strings.HasPrefix(arg, "--") {
			name := arg[2:]
			value := ""
			hasValue := false
			if eq := strings.IndexByte(name, '='); eq >= 0 {
				value = name[eq+1:]
				name = name[:eq]
				hasValue = true
			}
			switch name {
			case "help":
				return cfg, nil, actionHelp, nil
			case "version":
				return cfg, nil, actionVersion, nil
			case "quiet", "silent":
				cfg.headers = headerNever
			case "verbose":
				cfg.headers = headerAlways
			case "lines", "bytes":
				if !hasValue {
					i++
					if i >= len(args) {
						return cfg, nil, actionRun, &usageErr{fmt.Sprintf("option '--%s' requires an argument", name)}
					}
					value = args[i]
				}
				if err := applyCount(&cfg, name == "lines", value); err != nil {
					return cfg, nil, actionRun, err
				}
			default:
				return cfg, nil, actionRun, &usageErr{fmt.Sprintf("unrecognized option '--%s'", name)}
			}
			i++
			continue
		}

		// Short flags can be bunched up: "-n5", "-qv", "-n 5".
		j := 1
		for j < len(arg) {
			c := arg[j]
			switch c {
			case 'h':
				return cfg, nil, actionHelp, nil
			case 'q':
				cfg.headers = headerNever
				j++
			case 'v':
				cfg.headers = headerAlways
				j++
			case 'n', 'c':
				isLines := c == 'n'
				rest := arg[j+1:]
				rest = strings.TrimPrefix(rest, "=")
				var value string
				if rest != "" {
					value = rest
				} else {
					i++
					if i >= len(args) {
						return cfg, nil, actionRun, &usageErr{fmt.Sprintf("option requires an argument -- '%c'", c)}
					}
					value = args[i]
				}
				if err := applyCount(&cfg, isLines, value); err != nil {
					return cfg, nil, actionRun, err
				}
				j = len(arg) // the number used up the rest of the cluster
			default:
				return cfg, nil, actionRun, &usageErr{fmt.Sprintf("invalid option -- '%c'", c)}
			}
		}
		i++
	}

	return cfg, operands, actionRun, nil
}

func applyCount(cfg *config, isLines bool, value string) error {
	n, neg, err := parseCount(value)
	if err != nil {
		flag := "c"
		if isLines {
			flag = "n"
		}
		return &usageErr{fmt.Sprintf("invalid number of %s: '%s' (-%s)", unit(isLines), value, flag)}
	}
	if isLines {
		cfg.m = modeLines
	} else {
		cfg.m = modeBytes
	}
	cfg.count = n
	cfg.negative = neg
	return nil
}

func unit(isLines bool) string {
	if isLines {
		return "lines"
	}
	return "bytes"
}

// parseCount reads counts like "10", "-3" or "2k" (b=512, k=1K, m=1M) and
// returns the size plus whether it had a leading minus.
func parseCount(s string) (int64, bool, error) {
	if s == "" {
		return 0, false, fmt.Errorf("empty count")
	}
	neg := false
	body := s
	switch body[0] {
	case '+':
		body = body[1:]
	case '-':
		neg = true
		body = body[1:]
	}
	if body == "" {
		return 0, false, fmt.Errorf("missing digits")
	}

	multiplier := int64(1)
	last := body[len(body)-1]
	switch last {
	case 'b':
		multiplier = 512
		body = body[:len(body)-1]
	case 'k', 'K':
		multiplier = 1024
		body = body[:len(body)-1]
	case 'm', 'M':
		multiplier = 1024 * 1024
		body = body[:len(body)-1]
	}
	if body == "" {
		return 0, false, fmt.Errorf("missing digits before suffix")
	}

	n, err := strconv.ParseInt(body, 10, 64)
	if err != nil {
		return 0, false, err
	}
	if n < 0 {
		// Catches a doubled sign such as "--5".
		return 0, false, fmt.Errorf("negative digits")
	}
	if multiplier != 1 && n > math.MaxInt64/multiplier {
		return 0, false, fmt.Errorf("count overflow")
	}
	return n * multiplier, neg, nil
}

// friendlyOpenErr words open errors the way GNU head does.
func friendlyOpenErr(name string, err error) string {
	if os.IsNotExist(err) {
		return fmt.Sprintf("cannot open '%s' for reading: No such file or directory", name)
	}
	if os.IsPermission(err) {
		return fmt.Sprintf("cannot open '%s' for reading: Permission denied", name)
	}
	return fmt.Sprintf("cannot open '%s' for reading: %v", name, err)
}
