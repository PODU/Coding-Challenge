package comm

import (
	"fmt"
	"strings"
)

const usageText = `Usage: comm [OPTION]... FILE1 FILE2

Compare sorted files FILE1 and FILE2 line by line.
With no options, produce three tab-separated columns: lines only in FILE1,
lines only in FILE2, and lines common to both.

Options:
  -1                      suppress column 1 (lines unique to FILE1)
  -2                      suppress column 2 (lines unique to FILE2)
  -3                      suppress column 3 (lines common to both)
                          (flags may be combined, e.g. -12, -123)
  -i                      case-insensitive line comparison
      --check-order       check that the input is correctly sorted, even
                          if all input lines are pairable
      --nocheck-order     do not check that the input is correctly sorted
      --output-delimiter=STR  separate columns with STR instead of a tab
      --total             output a summary of line counts
      --help              display this help and exit

FILE may be '-' to read standard input (at most one input).
Exit status: 0 on success, 1 on error.
`

// errHelp means --help was given: print usage to stdout and exit 0.
var errHelp = fmt.Errorf("help requested")

// usageError is a bad command line; the usage text is printed after it.
type usageError struct{ msg string }

func (e *usageError) Error() string { return e.msg }

// Parse turns argv (without the program name) into a Config.
//
// The flag package can't handle clustered digit flags like -12 or "-" as an
// operand, so this is parsed by hand.
func Parse(args []string) (Config, error) {
	cfg := Config{delimiter: "\t"}

	var positionals []string
	noMoreOptions := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if noMoreOptions {
			positionals = append(positionals, arg)
			continue
		}

		switch {
		case arg == "--":
			noMoreOptions = true
		case arg == "--help":
			return Config{}, errHelp
		case arg == "--check-order":
			cfg.checkOrder = true
		case arg == "--nocheck-order":
			cfg.noCheckOrder = true
		case arg == "--total":
			cfg.total = true
		case arg == "--output-delimiter" || strings.HasPrefix(arg, "--output-delimiter="):
			if !strings.Contains(arg, "=") {
				return Config{}, &usageError{"option '--output-delimiter' requires an argument"}
			}
			val := arg[len("--output-delimiter="):]
			if val == "" {
				return Config{}, &usageError{"empty --output-delimiter not allowed"}
			}
			cfg.delimiter = val
		case strings.HasPrefix(arg, "--"):
			return Config{}, &usageError{fmt.Sprintf("unknown option %q", arg)}
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			if arg == "-i" {
				cfg.caseInsensitive = true
				break
			}
			body := arg[1:]
			if !isDigitFlags(body) {
				return Config{}, &usageError{fmt.Sprintf("unknown option %q", arg)}
			}
			for _, c := range body {
				cfg.suppress[c-'0'] = true
			}
		default:
			positionals = append(positionals, arg)
		}
	}

	if cfg.checkOrder && cfg.noCheckOrder {
		return Config{}, &usageError{"--check-order and --nocheck-order are mutually exclusive"}
	}

	if len(positionals) != 2 {
		return Config{}, &usageError{fmt.Sprintf("expected 2 file operands, got %d", len(positionals))}
	}

	cfg.file1 = positionals[0]
	cfg.file2 = positionals[1]

	if cfg.file1 == "-" && cfg.file2 == "-" {
		return Config{}, &usageError{"only one input may be standard input ('-')"}
	}

	return cfg, nil
}

// isDigitFlags reports whether s is a non-empty run of the digits 1-3.
func isDigitFlags(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '1' || c > '3' {
			return false
		}
	}
	return true
}
