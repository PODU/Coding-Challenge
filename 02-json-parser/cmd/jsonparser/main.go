// Command jsonparser checks whether a file (or stdin) is valid RFC 8259 JSON.
// Exit code 0 means valid, 1 means invalid or unreadable.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/codingchallenges/jsonparser/internal/parser"
)

// Overridable at build time via -ldflags.
var version = "1.0.0"

const usage = `jsonparser - validate JSON per RFC 8259

Usage:
  jsonparser [options] [file]

Arguments:
  file            Path to a JSON file. If omitted or "-", read from stdin.

Options:
  -q, --quiet     Suppress success/error text; communicate only via exit code.
  -v, --verbose   Print a summary of the parsed value on success.
      --version   Print version and exit 0.
  -h, --help      Print this help and exit 0.

Exit codes:
  0   Valid JSON.
  1   Invalid JSON (syntax error) or I/O error.
  2   Usage error (bad flags).
`

type options struct {
	quiet   bool
	verbose bool
	file    string // "" or "-" means stdin
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run returns the exit code instead of calling os.Exit so tests can drive it.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) (code int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(stderr, "Invalid JSON: internal error: %v\n", r)
			code = 1
		}
	}()

	opts, action := parseArgs(args)
	switch action {
	case actionHelp:
		fmt.Fprint(stdout, usage)
		return 0
	case actionVersion:
		fmt.Fprintf(stdout, "jsonparser %s\n", version)
		return 0
	case actionUsageError:
		fmt.Fprint(stderr, usage)
		return 2
	}

	data, err := readInput(opts.file, stdin)
	if err != nil {
		if !opts.quiet {
			fmt.Fprintf(stderr, "Error: %v\n", err)
		}
		return 1
	}

	value, perr := parser.Parse(data)
	if perr != nil {
		if !opts.quiet {
			fmt.Fprintf(stderr, "Invalid JSON: %v\n", perr)
		}
		return 1
	}

	if !opts.quiet {
		fmt.Fprintln(stdout, "Valid JSON")
		if opts.verbose {
			fmt.Fprintln(stdout, summarize(value))
		}
	}
	return 0
}

type action int

const (
	actionValidate action = iota
	actionHelp
	actionVersion
	actionUsageError
)

func parseArgs(args []string) (options, action) {
	var opts options
	fileSet := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			return opts, actionHelp
		case arg == "--version":
			return opts, actionVersion
		case arg == "-q" || arg == "--quiet":
			opts.quiet = true
		case arg == "-v" || arg == "--verbose":
			opts.verbose = true
		case arg == "-":
			if fileSet {
				return opts, actionUsageError
			}
			opts.file = "-"
			fileSet = true
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			return opts, actionUsageError
		default:
			if fileSet {
				return opts, actionUsageError
			}
			opts.file = arg
			fileSet = true
		}
	}
	return opts, actionValidate
}

func readInput(file string, stdin io.Reader) ([]byte, error) {
	if file == "" || file == "-" {
		return io.ReadAll(stdin)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no such file: %s", file)
		}
		return nil, err
	}
	return data, nil
}

func summarize(v parser.Value) string {
	switch n := v.(type) {
	case *parser.Object:
		return fmt.Sprintf("object with %d member(s)", len(n.Members))
	case *parser.Array:
		return fmt.Sprintf("array with %d element(s)", len(n.Elements))
	case *parser.String:
		return fmt.Sprintf("string (%d char(s))", len([]rune(n.Value)))
	case *parser.Number:
		return fmt.Sprintf("number %s", n.Raw)
	case *parser.Bool:
		return fmt.Sprintf("boolean %v", n.Value)
	case *parser.Null:
		return "null"
	default:
		return "value"
	}
}
