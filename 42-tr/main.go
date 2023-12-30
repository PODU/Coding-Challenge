// Command tr is a reimplementation of the portable parts of GNU tr: it copies
// stdin to stdout, translating, deleting or squeezing characters.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/podu2997/tr/internal/set"
	"github.com/podu2997/tr/internal/tr"
)

const versionText = "tr (Coding Challenge #42) 1.0.0\n"

const helpText = `Usage: tr [OPTION]... SET1 [SET2]
Translate, squeeze, and/or delete characters from standard input,
writing to standard output.

Options:
  -c, -C, --complement      use the complement of SET1
  -d, --delete              delete characters in SET1, do not translate
  -s, --squeeze-repeats     replace each sequence of a repeated character
                            that is listed in the last SET with one occurrence
  -t, --truncate-set1       first truncate SET1 to length of SET2
  -u                        unbuffered output (flush after every write)
  -h, --help                display this help and exit
  -v, --version             output version information and exit

Operands:
  SET1, SET2  strings of characters; may contain ranges (a-z),
              classes ([:alpha:]), escapes (\n \t \\ \NNN), and
              for SET2 the repeat form [c*n] / [c*].
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run runs tr and returns the exit code.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var fDelete, fSqueeze, fComplement, fTruncate, fUnbuffered bool
	var operands []string
	endFlags := false

	for i := 0; i < len(args); i++ {
		a := args[i]
		if endFlags {
			operands = append(operands, a)
			continue
		}
		if a == "--" {
			endFlags = true
			continue
		}
		if len(a) > 1 && a[0] == '-' {
			if strings.HasPrefix(a, "--") {
				switch a {
				case "--delete":
					fDelete = true
				case "--squeeze-repeats":
					fSqueeze = true
				case "--complement":
					fComplement = true
				case "--truncate-set1":
					fTruncate = true
				case "--help":
					fmt.Fprint(stdout, helpText)
					return 0
				case "--version":
					fmt.Fprint(stdout, versionText)
					return 0
				default:
					fmt.Fprintf(stderr, "tr: unrecognized option '%s'\n", a)
					fmt.Fprintln(stderr, "Try 'tr --help' for more information.")
					return 1
				}
			} else {
				for _, c := range a[1:] {
					switch c {
					case 'd':
						fDelete = true
					case 's':
						fSqueeze = true
					case 'c', 'C':
						fComplement = true
					case 't':
						fTruncate = true
					case 'u':
						fUnbuffered = true
					case 'h':
						fmt.Fprint(stdout, helpText)
						return 0
					case 'v':
						fmt.Fprint(stdout, versionText)
						return 0
					default:
						fmt.Fprintf(stderr, "tr: invalid option -- '%c'\n", c)
						fmt.Fprintln(stderr, "Try 'tr --help' for more information.")
						return 1
					}
				}
			}
			continue
		}
		operands = append(operands, a)
	}

	if len(operands) == 0 {
		fmt.Fprintln(stderr, "tr: missing operand")
		fmt.Fprintln(stderr, "Try 'tr --help' for more information.")
		return 1
	}
	if len(operands) > 2 {
		fmt.Fprintf(stderr, "tr: extra operand '%s'\n", operands[2])
		return 1
	}
	hasSet2 := len(operands) == 2

	// -s on its own takes one SET (squeeze) or two (translate, then squeeze).
	switch {
	case !fDelete && !fSqueeze:
		if !hasSet2 {
			fmt.Fprintf(stderr, "tr: missing operand after '%s'\n", operands[0])
			fmt.Fprintln(stderr, "Two strings must be given when translating.")
			return 1
		}
	case fDelete && !fSqueeze:
		if hasSet2 {
			fmt.Fprintf(stderr, "tr: extra operand '%s'\n", operands[1])
			return 1
		}
	case fDelete && fSqueeze:
		if !hasSet2 {
			fmt.Fprintln(stderr, "tr: missing operand")
			fmt.Fprintln(stderr, "Two strings must be given when deleting and squeezing.")
			return 1
		}
	}

	s1, err := set.Parse(operands[0])
	if err != nil {
		fmt.Fprintf(stderr, "tr: %s\n", err)
		return 1
	}
	var s2 *set.Set
	if hasSet2 {
		s2, err = set.Parse(operands[1])
		if err != nil {
			fmt.Fprintf(stderr, "tr: %s\n", err)
			return 1
		}
	}

	eng, err := tr.Build(tr.Config{
		Delete:     fDelete,
		Squeeze:    fSqueeze,
		Complement: fComplement,
		Truncate:   fTruncate,
		Unbuffered: fUnbuffered,
		Set1:       s1,
		Set2:       s2,
		HasSet2:    hasSet2,
	})
	if err != nil {
		fmt.Fprintf(stderr, "tr: %s\n", err)
		return 1
	}

	if err := eng.Run(stdin, stdout); err != nil {
		fmt.Fprintf(stderr, "tr: %s\n", err)
		return 1
	}
	return 0
}
