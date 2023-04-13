// Package cli parses cut's command-line arguments into a validated cut.Config.
//
// cut uses a small, idiosyncratic flag grammar (attached values like -f2 and
// -d,, clustered short flags like -sn, and GNU long options), so it is parsed
// by hand rather than with the standard flag package.
package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/codingchallenges/cut/internal/cut"
)

const Version = "cut (coding-challenges) 1.0.0"

// ErrHelp and ErrVersion are returned for --help and --version; main prints
// the text and exits 0.
var (
	ErrHelp    = errors.New("help requested")
	ErrVersion = errors.New("version requested")
)

// UsageError is a bad command line. main prints it and exits 2.
type UsageError struct{ msg string }

func (e *UsageError) Error() string { return e.msg }

func usagef(format string, a ...any) *UsageError {
	return &UsageError{msg: fmt.Sprintf(format, a...)}
}

const HelpText = `Usage: cut OPTION... [FILE...]
Print selected parts of lines from each FILE to standard output.
With no FILE, or when FILE is -, read standard input.

Mandatory (choose exactly one):
  -b, --bytes=LIST          select only these bytes
  -c, --characters=LIST     select only these characters
  -f, --fields=LIST         select only these fields; also print any line
                              that contains no delimiter character, unless
                              the -s option is specified

Options:
  -d, --delimiter=DELIM     use DELIM instead of TAB as the field delimiter
  -s, --only-delimited      do not print lines not containing delimiters
      --complement          complement the set of selected bytes, characters
                              or fields
      --output-delimiter=STRING  use STRING as the output delimiter
                              the default is to use the input delimiter
  -n                        (with -b) do not split multibyte characters
      --help                display this help and exit
      --version             output version information and exit

LIST is a comma- or whitespace-separated list of: N, N-M, N-, -M (1-indexed).
`

// modeState remembers which of -b/-c/-f was given so a second one can be rejected.
type modeState struct {
	set  bool
	mode cut.Mode
	list string
}

// Parse turns args (without the program name) into a cut.Config. Errors are
// ErrHelp, ErrVersion or a *UsageError.
func Parse(args []string) (cut.Config, error) {
	var (
		ms         modeState
		delim      byte = '\t'
		delimSet   bool
		outDelim   string
		outSet     bool
		onlyDelim  bool
		complement bool
		files      []string
		noOpts     bool // after "--", everything is a file
	)

	setMode := func(m cut.Mode, list string) error {
		if ms.set {
			return usagef("only one type of list may be specified")
		}
		ms = modeState{set: true, mode: m, list: list}
		return nil
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		switch {
		case noOpts || arg == "-" || arg == "" || arg[0] != '-':
			files = append(files, arg)

		case arg == "--":
			noOpts = true

		case strings.HasPrefix(arg, "--"):
			name, value, hasValue := strings.Cut(arg[2:], "=")
			takeValue := func() (string, error) {
				if hasValue {
					return value, nil
				}
				if i+1 >= len(args) {
					return "", usagef("option '--%s' requires an argument", name)
				}
				i++
				return args[i], nil
			}
			switch name {
			case "help":
				return cut.Config{}, ErrHelp
			case "version":
				return cut.Config{}, ErrVersion
			case "bytes":
				v, err := takeValue()
				if err != nil {
					return cut.Config{}, err
				}
				if err := setMode(cut.ModeBytes, v); err != nil {
					return cut.Config{}, err
				}
			case "characters":
				v, err := takeValue()
				if err != nil {
					return cut.Config{}, err
				}
				if err := setMode(cut.ModeChars, v); err != nil {
					return cut.Config{}, err
				}
			case "fields":
				v, err := takeValue()
				if err != nil {
					return cut.Config{}, err
				}
				if err := setMode(cut.ModeFields, v); err != nil {
					return cut.Config{}, err
				}
			case "delimiter":
				v, err := takeValue()
				if err != nil {
					return cut.Config{}, err
				}
				b, err := parseDelim(v)
				if err != nil {
					return cut.Config{}, err
				}
				delim, delimSet = b, true
			case "output-delimiter":
				v, err := takeValue()
				if err != nil {
					return cut.Config{}, err
				}
				outDelim, outSet = v, true
			case "only-delimited":
				onlyDelim = true
			case "complement":
				complement = true
			default:
				return cut.Config{}, usagef("unrecognized option '--%s'", name)
			}

		default:
			// Short options, possibly clustered (-sf2).
			tok := arg[1:]
			j := 0
			for j < len(tok) {
				ch := tok[j]
				switch ch {
				case 's':
					onlyDelim = true
					j++
				case 'n':
					// Accepted for compatibility; characters are never split anyway.
					j++
				case 'f', 'b', 'c', 'd':
					// The value is the rest of this token, or the next argument.
					val := tok[j+1:]
					if val == "" {
						if i+1 >= len(args) {
							return cut.Config{}, usagef("option requires an argument -- '%c'", ch)
						}
						i++
						val = args[i]
					}
					switch ch {
					case 'f':
						if err := setMode(cut.ModeFields, val); err != nil {
							return cut.Config{}, err
						}
					case 'b':
						if err := setMode(cut.ModeBytes, val); err != nil {
							return cut.Config{}, err
						}
					case 'c':
						if err := setMode(cut.ModeChars, val); err != nil {
							return cut.Config{}, err
						}
					case 'd':
						b, err := parseDelim(val)
						if err != nil {
							return cut.Config{}, err
						}
						delim, delimSet = b, true
					}
					j = len(tok)
				default:
					return cut.Config{}, usagef("invalid option -- '%c'", ch)
				}
			}
		}
	}

	if !ms.set {
		return cut.Config{}, usagef("you must specify a list of bytes, characters, or fields")
	}
	if ms.mode != cut.ModeFields {
		if delimSet {
			return cut.Config{}, usagef("an input delimiter may be specified only when operating on fields")
		}
		if onlyDelim {
			return cut.Config{}, usagef("suppressing non-delimited lines makes sense\n\tonly when operating on fields")
		}
	}

	sel, err := cut.ParseList(ms.list, complement)
	if err != nil {
		return cut.Config{}, &UsageError{msg: err.Error()}
	}

	if len(files) == 0 {
		files = []string{"-"}
	}

	return cut.Config{
		Mode:        ms.mode,
		Sel:         sel,
		Delim:       delim,
		OutDelim:    outDelim,
		OutDelimSet: outSet,
		OnlyDelim:   onlyDelim,
		Files:       files,
	}, nil
}

// parseDelim accepts only a single-byte delimiter; multi-byte UTF-8
// delimiters are not supported.
func parseDelim(s string) (byte, error) {
	if len(s) != 1 {
		return 0, usagef("the delimiter must be a single character")
	}
	return s[0], nil
}
