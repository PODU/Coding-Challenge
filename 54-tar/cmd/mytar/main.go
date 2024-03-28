// Command mytar creates (-c), extracts (-x) and lists (-t) POSIX USTAR
// archives, using a file (-f) or stdin/stdout. archive/tar is only used in
// tests.
package main

import (
	"fmt"
	"io"
	"os"

	"mytar/internal/cmd"
)

const usage = `usage: mytar -c|-x|-t [-v] [-f ARCHIVE] [FILE...]

  -c            create a new archive from FILE...
  -x            extract members from the archive (into the current directory)
  -t            list member names (with -v: long format)
  -f ARCHIVE    use ARCHIVE; "-" or omitted means stdout (create) / stdin (x,t)
  -v            verbose

Bundled flags are accepted, e.g. mytar -cvf out.tar dir/`

type options struct {
	create, extract, list, verbose bool
	file                           string
	args                           []string
}

func main() {
	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "mytar: %v\n\n%s\n", err, usage)
		os.Exit(2)
	}
	if err := run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "mytar: %v\n", err)
		os.Exit(1)
	}
}

// parseArgs accepts bundled flags (-cvf out.tar) as well as separate ones and
// requires exactly one mode.
func parseArgs(argv []string) (options, error) {
	var o options
	var positionals []string
	expectFile := false

	for _, a := range argv {
		if expectFile {
			o.file = a
			expectFile = false
			continue
		}
		if len(a) >= 1 && a[0] == '-' && a != "-" {
			for i := 1; i < len(a); i++ {
				switch a[i] {
				case 'c':
					o.create = true
				case 'x':
					o.extract = true
				case 't':
					o.list = true
				case 'v':
					o.verbose = true
				case 'f':
					// -fNAME or -f NAME
					if i+1 < len(a) {
						o.file = a[i+1:]
					} else {
						expectFile = true
					}
					i = len(a)
				default:
					return o, fmt.Errorf("unknown flag -%c", a[i])
				}
			}
			continue
		}
		positionals = append(positionals, a)
	}
	if expectFile {
		return o, fmt.Errorf("-f requires an argument")
	}
	o.args = positionals

	modes := 0
	if o.create {
		modes++
	}
	if o.extract {
		modes++
	}
	if o.list {
		modes++
	}
	if modes != 1 {
		return o, fmt.Errorf("exactly one of -c, -x, -t must be supplied")
	}
	return o, nil
}

func run(o options) error {
	switch {
	case o.create:
		w, closeFn, err := openOut(o.file)
		if err != nil {
			return err
		}
		defer closeFn()
		return cmd.Create(w, o.args, o.verbose, os.Stderr)
	case o.list:
		r, closeFn, err := openIn(o.file)
		if err != nil {
			return err
		}
		defer closeFn()
		return cmd.List(r, os.Stdout, o.verbose)
	case o.extract:
		r, closeFn, err := openIn(o.file)
		if err != nil {
			return err
		}
		defer closeFn()
		dest, err := os.Getwd()
		if err != nil {
			return err
		}
		return cmd.Extract(r, dest, o.verbose, os.Stderr)
	}
	return fmt.Errorf("no mode selected")
}

func openOut(file string) (io.Writer, func(), error) {
	if file == "" || file == "-" {
		return os.Stdout, func() {}, nil
	}
	f, err := os.Create(file)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}

func openIn(file string) (io.Reader, func(), error) {
	if file == "" || file == "-" {
		return os.Stdin, func() {}, nil
	}
	f, err := os.Open(file)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}
