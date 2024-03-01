package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
)

const usage = `Usage: ccxargs [OPTIONS] [command [initial-arguments...]]

Read items from standard input (or -a FILE), build command lines, and run them.

Options:
  -0, --null               Items are NUL-separated; disable quote/space splitting.
  -d, --delimiter CHAR     Use CHAR (or escape \n \t \r \0 \\) as the sole delimiter.
  -n, --max-args N         At most N items per command invocation (N >= 1).
  -s, --max-chars N        At most N bytes per constructed command line.
  -I, --replace[=R]        Replace token R (default {}) in args with each input
                           item; implies one command per item.
  -P, --max-procs N        Run up to N invocations in parallel (0 = max). Default 1.
  -a, --arg-file FILE      Read items from FILE instead of stdin.
  -r, --no-run-if-empty    Do not run the command if the input is empty.
  -t, --verbose            Print each command to stderr before running it.
  -E EOF                   Stop reading at an item equal to EOF (whitespace mode).
  -h, --help               Show this help.

If command is omitted, echo (cmd /c echo on Windows) is used.
`

type options struct {
	null        bool
	delim       byte
	delimSet    bool
	maxArgs     int
	maxChars    int
	maxCharsSet bool
	replace     bool
	replaceStr  string
	maxProcs    int
	argFile     string
	argFileSet  bool
	noRunEmpty  bool
	verbose     bool
	eofStr      string
	eofSet      bool
	help        bool
	command     []string
}

func main() {
	opt, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}
	if opt.help {
		fmt.Print(usage)
		os.Exit(0)
	}
	os.Exit(run(opt))
}

func run(opt options) int {
	var input io.Reader = os.Stdin
	if opt.argFileSet {
		f, err := os.Open(opt.argFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ccxargs: %v\n", err)
			return 1
		}
		defer f.Close()
		input = f
	}

	tcfg := TokenizerConfig{EOFString: opt.eofStr, EOFSet: opt.eofSet}
	switch {
	case opt.delimSet:
		tcfg.Mode = ModeDelimiter
		tcfg.Delimiter = opt.delim
	case opt.null:
		tcfg.Mode = ModeNull
	default:
		tcfg.Mode = ModeWhitespace
	}

	items, err := TokenizeAll(input, tcfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	prefix := opt.command
	if len(prefix) == 0 {
		prefix = defaultCommand()
	}

	maxChars := opt.maxChars
	if !opt.maxCharsSet {
		maxChars = defaultMaxChars()
	}

	jobs := Build(BuildConfig{
		Prefix:     prefix,
		MaxArgs:    opt.maxArgs,
		MaxChars:   maxChars,
		Replace:    opt.replace,
		ReplaceStr: opt.replaceStr,
		RunIfEmpty: !opt.noRunEmpty,
	}, items)
	return RunJobs(jobs, ExecConfig{MaxProcs: opt.maxProcs, Verbose: opt.verbose})
}

// defaultCommand is echo. On Windows that is a cmd.exe builtin, not a program.
func defaultCommand() []string {
	if runtime.GOOS == "windows" {
		return []string{"cmd", "/c", "echo"}
	}
	return []string{"echo"}
}

// defaultMaxChars is the -s default, kept well under the OS limit.
func defaultMaxChars() int {
	if runtime.GOOS == "windows" {
		return 30000 // CreateProcess caps the command line near 32 KiB.
	}
	return 131072
}

// parseArgs parses options up to the first non-option argument (or "--"),
// which starts the command. Everything from there on belongs to the command.
func parseArgs(args []string) (options, error) {
	opt := options{maxProcs: 1, replaceStr: "{}"}

	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--":
			opt.command = append([]string{}, args[i+1:]...)
			return opt, nil
		case a == "-":
			// A bare "-" is taken as the command name.
			opt.command = append([]string{}, args[i:]...)
			return opt, nil
		case strings.HasPrefix(a, "--"):
			consumed, err := parseLong(&opt, args, i)
			if err != nil {
				return opt, err
			}
			if opt.command != nil || opt.help {
				return opt, nil
			}
			i += consumed
		case strings.HasPrefix(a, "-"):
			consumed, err := parseShortCluster(&opt, args, i)
			if err != nil {
				return opt, err
			}
			if opt.help {
				return opt, nil
			}
			i += consumed
		default:
			opt.command = append([]string{}, args[i:]...)
			return opt, nil
		}
	}
	return opt, nil
}

// parseLong parses "--name" or "--name=value" and returns how many arguments
// it used.
func parseLong(opt *options, args []string, i int) (int, error) {
	a := args[i]
	name := a[2:]
	val := ""
	hasVal := false
	if eq := strings.IndexByte(name, '='); eq >= 0 {
		val, name, hasVal = name[eq+1:], name[:eq], true
	}

	needValue := func() (string, int, error) {
		if hasVal {
			return val, 1, nil
		}
		if i+1 >= len(args) {
			return "", 0, fmt.Errorf("ccxargs: option --%s requires an argument", name)
		}
		return args[i+1], 2, nil
	}

	switch name {
	case "null":
		opt.null = true
		return 1, nil
	case "no-run-if-empty":
		opt.noRunEmpty = true
		return 1, nil
	case "verbose":
		opt.verbose = true
		return 1, nil
	case "help":
		opt.help = true
		return 1, nil
	case "replace":
		opt.replace = true
		if hasVal {
			if val == "" {
				return 0, errors.New("ccxargs: --replace string may not be empty")
			}
			opt.replaceStr = val
		}
		return 1, nil
	case "delimiter":
		v, c, err := needValue()
		if err != nil {
			return 0, err
		}
		b, err := parseDelimiter(v)
		if err != nil {
			return 0, err
		}
		opt.delim, opt.delimSet = b, true
		return c, nil
	case "max-args":
		v, c, err := needValue()
		if err != nil {
			return 0, err
		}
		n, err := parseIntAtLeast(v, "--max-args", 1)
		if err != nil {
			return 0, err
		}
		opt.maxArgs = n
		return c, nil
	case "max-chars":
		v, c, err := needValue()
		if err != nil {
			return 0, err
		}
		n, err := parseIntAtLeast(v, "--max-chars", 1)
		if err != nil {
			return 0, err
		}
		opt.maxChars, opt.maxCharsSet = n, true
		return c, nil
	case "max-procs":
		v, c, err := needValue()
		if err != nil {
			return 0, err
		}
		n, err := parseIntAtLeast(v, "--max-procs", 0)
		if err != nil {
			return 0, err
		}
		opt.maxProcs = n
		return c, nil
	case "arg-file":
		v, c, err := needValue()
		if err != nil {
			return 0, err
		}
		opt.argFile, opt.argFileSet = v, true
		return c, nil
	default:
		return 0, fmt.Errorf("ccxargs: unknown option --%s", name)
	}
}

// parseShortCluster parses a group of short options such as "-rt" or "-n2".
// An option that takes a value uses the rest of the group, or else the next
// argument.
func parseShortCluster(opt *options, args []string, i int) (consumed int, err error) {
	a := args[i]
	consumed = 1
	j := 1
	for j < len(a) {
		c := a[j]
		takeValue := func(flag string) (string, error) {
			rest := a[j+1:]
			if rest != "" {
				j = len(a)
				return rest, nil
			}
			if i+1 >= len(args) {
				return "", fmt.Errorf("ccxargs: option -%s requires an argument", flag)
			}
			consumed = 2
			j = len(a)
			return args[i+1], nil
		}

		switch c {
		case '0':
			opt.null = true
			j++
		case 'r':
			opt.noRunEmpty = true
			j++
		case 't':
			opt.verbose = true
			j++
		case 'h':
			opt.help = true
			return consumed, nil
		case 'd':
			v, e := takeValue("d")
			if e != nil {
				return 0, e
			}
			b, e := parseDelimiter(v)
			if e != nil {
				return 0, e
			}
			opt.delim, opt.delimSet = b, true
		case 'n':
			v, e := takeValue("n")
			if e != nil {
				return 0, e
			}
			n, e := parseIntAtLeast(v, "-n", 1)
			if e != nil {
				return 0, e
			}
			opt.maxArgs = n
		case 's':
			v, e := takeValue("s")
			if e != nil {
				return 0, e
			}
			n, e := parseIntAtLeast(v, "-s", 1)
			if e != nil {
				return 0, e
			}
			opt.maxChars, opt.maxCharsSet = n, true
		case 'P':
			v, e := takeValue("P")
			if e != nil {
				return 0, e
			}
			n, e := parseIntAtLeast(v, "-P", 0)
			if e != nil {
				return 0, e
			}
			opt.maxProcs = n
		case 'a':
			v, e := takeValue("a")
			if e != nil {
				return 0, e
			}
			opt.argFile, opt.argFileSet = v, true
		case 'E':
			v, e := takeValue("E")
			if e != nil {
				return 0, e
			}
			opt.eofStr, opt.eofSet = v, true
		case 'I':
			v, e := takeValue("I")
			if e != nil {
				return 0, e
			}
			if v == "" {
				return 0, errors.New("ccxargs: -I requires a non-empty replacement string")
			}
			opt.replace, opt.replaceStr = true, v
		default:
			return 0, fmt.Errorf("ccxargs: unknown option -%c", c)
		}
	}
	return consumed, nil
}

func parseIntAtLeast(s, flag string, min int) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("ccxargs: invalid value %q for %s", s, flag)
	}
	if n < min {
		return 0, fmt.Errorf("ccxargs: value for %s must be >= %d", flag, min)
	}
	return n, nil
}

// parseDelimiter accepts a single character or one of \n \t \r \0 \\.
func parseDelimiter(s string) (byte, error) {
	if len(s) == 1 {
		return s[0], nil
	}
	if len(s) == 2 && s[0] == '\\' {
		switch s[1] {
		case 'n':
			return '\n', nil
		case 't':
			return '\t', nil
		case 'r':
			return '\r', nil
		case '0':
			return 0, nil
		case '\\':
			return '\\', nil
		}
	}
	return 0, fmt.Errorf("ccxargs: -d expects a single character or escape (\\n \\t \\r \\0 \\\\), got %q", s)
}
