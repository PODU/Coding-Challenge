package main

import (
	"fmt"
	"os"
	"strings"
)

type Options struct {
	Quiet        bool
	InPlace      bool
	BackupSuffix string
	Script       string
	Files        []string
	ShowHelp     bool
}

type usageErr struct{ msg string }

func (e *usageErr) Error() string { return e.msg }

func usagef(format string, a ...any) error {
	return &usageErr{msg: fmt.Sprintf(format, a...)}
}

const usageText = `ccsed - a stream editor (Coding Challenge #21)

Usage:
  ccsed [OPTIONS] {script | -e script... | -f file...} [input-file...]

Options:
  -n            Quiet: suppress automatic printing of the pattern space.
  -e SCRIPT     Add SCRIPT to the commands to execute (repeatable).
  -f FILE       Add the contents of FILE to the commands (repeatable).
  -i[SUFFIX]    Edit files in place (back up to NAME+SUFFIX if SUFFIX given).
  --            Treat all following arguments as operands.
  -h, --help    Print this help and exit.

Commands:
  s/re/repl/[gpN]   substitute (g=all, p=print, N=Nth occurrence)
  p                 print the pattern space
  d                 delete the pattern space (end the cycle)
  G                 append a blank line

Addresses:
  N        line N            $          last line
  /re/     matching lines    A,B        range from A to B
  A,B!cmd  negated address
`

// ParseArgs parses argv (excluding the program name) into Options.
func ParseArgs(args []string) (*Options, error) {
	opts := &Options{}
	var scripts []string
	hasScript := false

	i := 0
	for i < len(args) {
		a := args[i]
		if a == "--" {
			i++
			break
		}
		if a == "-h" || a == "--help" {
			opts.ShowHelp = true
			return opts, nil
		}
		if a == "-" || !strings.HasPrefix(a, "-") {
			break
		}

		// Short options can be bundled, e.g. -ne.
		flags := a[1:]
		for len(flags) > 0 {
			switch flags[0] {
			case 'n':
				opts.Quiet = true
				flags = flags[1:]
			case 'e':
				rest := flags[1:]
				if rest == "" {
					i++
					if i >= len(args) {
						return nil, usagef("option -e requires an argument")
					}
					rest = args[i]
				}
				scripts = append(scripts, rest)
				hasScript = true
				flags = ""
			case 'f':
				rest := flags[1:]
				if rest == "" {
					i++
					if i >= len(args) {
						return nil, usagef("option -f requires an argument")
					}
					rest = args[i]
				}
				content, err := os.ReadFile(rest)
				if err != nil {
					return nil, usagef("cannot read script file %q: %v", rest, err)
				}
				scripts = append(scripts, string(content))
				hasScript = true
				flags = ""
			case 'i':
				opts.InPlace = true
				if len(flags) > 1 {
					opts.BackupSuffix = flags[1:]
				}
				flags = ""
			default:
				return nil, usagef("unknown option -%c", flags[0])
			}
		}
		i++
	}

	operands := args[i:]
	if !hasScript {
		if len(operands) == 0 {
			return nil, usagef("no script specified")
		}
		opts.Script = operands[0]
		opts.Files = operands[1:]
	} else {
		opts.Script = strings.Join(scripts, "\n")
		opts.Files = operands
	}
	return opts, nil
}
