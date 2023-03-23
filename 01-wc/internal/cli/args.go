// Package cli parses ccwc's flags and formats output rows like GNU wc.
package cli

import (
	"fmt"
	"io"
	"strings"
)

// Options is the parsed command line. A "-" in Files means stdin.
type Options struct {
	Lines bool
	Words bool
	Chars bool // -m
	Bytes bool // -c
	Files []string
}

// AnyCount reports whether at least one count flag was selected.
func (o Options) AnyCount() bool {
	return o.Lines || o.Words || o.Chars || o.Bytes
}

// ApplyDefaults selects lines, words and bytes when no count flag was given,
// as wc does.
func (o *Options) ApplyDefaults() {
	if !o.AnyCount() {
		o.Lines = true
		o.Words = true
		o.Bytes = true
	}
}

const usage = `Usage: ccwc [OPTION]... [FILE]...
Print line, word, character, and byte counts for each FILE, and a total line
if more than one FILE is specified. With no FILE, or when FILE is -, read
standard input.

Options:
  -c    print the byte count
  -l    print the line (newline) count
  -w    print the word count
  -m    print the character count (UTF-8 aware)
  -h    display this help and exit

With no count option, -l -w -c (lines, words, bytes) are printed.
When several count options are given, the column order is fixed:
lines, words, characters, bytes.
`

// UsageError is returned for an unknown flag. main exits with code 2 on it.
type UsageError struct{ msg string }

func (e *UsageError) Error() string { return e.msg }

type helpRequested struct{}

func (helpRequested) Error() string { return "help requested" }

// ErrHelp is returned by Parse for -h or --help.
var ErrHelp error = helpRequested{}

// Parse parses the arguments after the program name.
//
// The flag package can't handle bundled flags like -lw, which wc accepts, so
// this is a tiny hand-rolled getopt. "--" ends option parsing and a lone "-"
// is a filename.
func Parse(args []string) (Options, error) {
	var o Options
	filesOnly := false

	for _, arg := range args {
		switch {
		case filesOnly:
			o.Files = append(o.Files, arg)
		case arg == "--":
			filesOnly = true
		case arg == "-":
			o.Files = append(o.Files, arg)
		case arg == "--help" || arg == "-h":
			return Options{}, ErrHelp
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			for _, r := range arg[1:] {
				switch r {
				case 'l':
					o.Lines = true
				case 'w':
					o.Words = true
				case 'm':
					o.Chars = true
				case 'c':
					o.Bytes = true
				case 'h':
					return Options{}, ErrHelp
				default:
					return Options{}, &UsageError{
						msg: fmt.Sprintf("ccwc: invalid option -- '%c'", r),
					}
				}
			}
		default:
			o.Files = append(o.Files, arg)
		}
	}

	return o, nil
}

// PrintUsage writes the help text to w.
func PrintUsage(w io.Writer) {
	fmt.Fprint(w, usage)
}
