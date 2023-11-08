// Command ccjq is a small clone of the jq command-line JSON processor.
//
//	ccjq [OPTIONS] [FILTER] [FILES...]
//
// See README.md for the supported filter subset and flags.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"ccjq/internal/jq"
)

const version = "ccjq 0.1.0 (jq-compatible subset)"

const usage = `Usage: ccjq [OPTIONS] [FILTER] [FILES...]

A small clone of jq. Reads JSON, applies a filter, writes JSON.

Options:
  -r, --raw-output      Output strings without JSON quotes.
  -j, --join-output     Like -r but without a trailing newline.
  -c, --compact-output  Compact instead of pretty-printed output.
  -n, --null-input      Use null as the single input value.
  -s, --slurp           Read all inputs into one array.
  -e, --exit-status     Set exit code from the last output value.
  -S, --sort-keys       Sort object keys in the output.
  -a, --ascii-output    Escape non-ASCII characters as \uXXXX.
      --tab             Indent with tabs.
      --indent N        Indent with N spaces (default 2).
  -h, --help            Show this help.
  -V, --version         Show version.
`

type options struct {
	raw       bool
	join      bool
	compact   bool
	nullInput bool
	slurp     bool
	exitStat  bool
	sortKeys  bool
	ascii     bool
	indent    string
	filter    string
	hasFilter bool
	files     []string
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, code, done := parseArgs(args, stderr)
	if done {
		return code
	}

	prog, err := jq.Compile(opts.filter)
	if err != nil {
		fmt.Fprintf(stderr, "ccjq: error: %s\n", err)
		return 3
	}

	out := bufio.NewWriter(stdout)
	defer out.Flush()

	enc := jq.EncodeOptions{
		Compact:  opts.compact,
		Raw:      opts.raw || opts.join,
		SortKeys: opts.sortKeys,
		ASCII:    opts.ascii,
		Indent:   opts.indent,
	}

	state := &runState{out: out, enc: enc, join: opts.join}

	exitCode := 0
	emit := func(v jq.Value) error {
		state.lastOutput = v
		state.produced = true
		s := jq.Encode(v, enc)
		if _, err := out.WriteString(s); err != nil {
			return err
		}
		if !opts.join {
			if err := out.WriteByte('\n'); err != nil {
				return err
			}
		}
		return nil
	}

	processOne := func(input jq.Value) {
		if err := jq.Run(prog, input, emit); err != nil {
			out.Flush()
			fmt.Fprintf(stderr, "ccjq: error: %s\n", err)
			exitCode = 5
		}
	}

	switch {
	case opts.nullInput:
		processOne(nil)
	case opts.slurp:
		arr, derr := slurp(opts, stdin)
		if derr != nil {
			out.Flush()
			fmt.Fprintf(stderr, "ccjq: error: %s\n", derr)
			return 2
		}
		processOne(arr)
	default:
		reader, closeFn, oerr := inputReader(opts, stdin)
		if oerr != nil {
			fmt.Fprintf(stderr, "ccjq: error: %s\n", oerr)
			return 2
		}
		dec := jq.NewDecoder(reader)
		for {
			v, derr := dec.Decode()
			if derr == io.EOF {
				break
			}
			if derr != nil {
				out.Flush()
				fmt.Fprintf(stderr, "ccjq: error (while parsing): %s\n", derr)
				exitCode = 2
				break
			}
			processOne(v)
		}
		if closeFn != nil {
			closeFn()
		}
	}

	if err := out.Flush(); err != nil {
		fmt.Fprintf(stderr, "ccjq: error: %s\n", err)
		return 2
	}

	if exitCode != 0 {
		return exitCode
	}
	if opts.exitStat {
		if !state.produced {
			return 4
		}
		if !jq.Truthy(state.lastOutput) {
			return 1
		}
	}
	return 0
}

type runState struct {
	out        *bufio.Writer
	enc        jq.EncodeOptions
	join       bool
	lastOutput jq.Value
	produced   bool
}

func parseArgs(args []string, stderr io.Writer) (options, int, bool) {
	opts := options{indent: "  "}
	endFlags := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if endFlags || a == "-" || !strings.HasPrefix(a, "-") {
			if !opts.hasFilter {
				opts.filter = a
				opts.hasFilter = true
			} else {
				opts.files = append(opts.files, a)
			}
			continue
		}
		if a == "--" {
			endFlags = true
			continue
		}
		if strings.HasPrefix(a, "--") {
			switch a {
			case "--raw-output":
				opts.raw = true
			case "--join-output":
				opts.join = true
			case "--compact-output":
				opts.compact = true
			case "--null-input":
				opts.nullInput = true
			case "--slurp":
				opts.slurp = true
			case "--exit-status":
				opts.exitStat = true
			case "--sort-keys":
				opts.sortKeys = true
			case "--ascii-output":
				opts.ascii = true
			case "--tab":
				opts.indent = "\t"
			case "--indent":
				if i+1 >= len(args) {
					fmt.Fprintln(stderr, "ccjq: --indent requires a value")
					return opts, 2, true
				}
				i++
				n, err := strconv.Atoi(args[i])
				if err != nil || n < 0 || n > 8 {
					fmt.Fprintln(stderr, "ccjq: --indent value must be between 0 and 8")
					return opts, 2, true
				}
				opts.indent = strings.Repeat(" ", n)
			case "--help":
				fmt.Fprint(stderr, usage)
				return opts, 0, true
			case "--version":
				fmt.Fprintln(stderr, version)
				return opts, 0, true
			default:
				fmt.Fprintf(stderr, "ccjq: unknown option %q\n", a)
				return opts, 2, true
			}
			continue
		}
		// short flag cluster, e.g. -rc
		for j := 1; j < len(a); j++ {
			switch a[j] {
			case 'r':
				opts.raw = true
			case 'j':
				opts.join = true
			case 'c':
				opts.compact = true
			case 'n':
				opts.nullInput = true
			case 's':
				opts.slurp = true
			case 'e':
				opts.exitStat = true
			case 'S':
				opts.sortKeys = true
			case 'a':
				opts.ascii = true
			case 'h':
				fmt.Fprint(stderr, usage)
				return opts, 0, true
			case 'V':
				fmt.Fprintln(stderr, version)
				return opts, 0, true
			default:
				fmt.Fprintf(stderr, "ccjq: unknown option -%c\n", a[j])
				return opts, 2, true
			}
		}
	}
	if !opts.hasFilter {
		opts.filter = "."
	}
	return opts, 0, false
}

// inputReader returns a single reader over stdin or the concatenation of files.
func inputReader(opts options, stdin io.Reader) (io.Reader, func(), error) {
	if len(opts.files) == 0 {
		return stdin, nil, nil
	}
	var readers []io.Reader
	var closers []io.Closer
	for _, name := range opts.files {
		f, err := os.Open(name)
		if err != nil {
			for _, c := range closers {
				c.Close()
			}
			return nil, nil, err
		}
		readers = append(readers, f)
		readers = append(readers, strings.NewReader("\n"))
		closers = append(closers, f)
	}
	closeFn := func() {
		for _, c := range closers {
			c.Close()
		}
	}
	return io.MultiReader(readers...), closeFn, nil
}

// slurp reads the whole input stream into one array.
func slurp(opts options, stdin io.Reader) (jq.Value, error) {
	reader, closeFn, err := inputReader(opts, stdin)
	if err != nil {
		return nil, err
	}
	if closeFn != nil {
		defer closeFn()
	}
	dec := jq.NewDecoder(reader)
	arr := []jq.Value{}
	for {
		v, derr := dec.Decode()
		if derr == io.EOF {
			break
		}
		if derr != nil {
			return nil, derr
		}
		arr = append(arr, v)
	}
	return arr, nil
}
