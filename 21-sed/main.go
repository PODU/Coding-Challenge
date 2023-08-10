package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	opts, err := ParseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "ccsed: %s\n", err)
		return 2
	}
	if opts.ShowHelp {
		fmt.Fprint(stdout, usageText)
		return 0
	}

	// `#n` as the very first line of the script implies -n.
	if opts.Script == "#n" || strings.HasPrefix(opts.Script, "#n\n") {
		opts.Quiet = true
	}

	cmds, err := Parse(opts.Script)
	if err != nil {
		fmt.Fprintf(stderr, "ccsed: parse error: %s\n", err)
		return 2
	}

	eng := &Engine{Commands: cmds, Quiet: opts.Quiet}

	if opts.InPlace {
		if len(opts.Files) == 0 {
			fmt.Fprintln(stderr, "ccsed: -i may not be used with stdin; specify file(s)")
			return 2
		}
		exit := 0
		for _, f := range opts.Files {
			if f == "-" {
				fmt.Fprintln(stderr, "ccsed: -i may not be used with stdin")
				exit = 2
				continue
			}
			if err := eng.RunInPlace(f, opts.BackupSuffix); err != nil {
				fmt.Fprintf(stderr, "ccsed: %s: %v\n", f, err)
				exit = 1
			}
		}
		return exit
	}

	if len(opts.Files) == 0 {
		if err := eng.Run(stdin, stdout); err != nil {
			fmt.Fprintf(stderr, "ccsed: %v\n", err)
			return 1
		}
		return 0
	}

	// Concatenate all files into a single stream so that ranges and `$` span
	// the whole input (sed's default, non -s, behaviour).
	var readers []io.Reader
	var closers []io.Closer
	exit := 0
	for _, f := range opts.Files {
		if f == "-" {
			readers = append(readers, stdin)
			continue
		}
		fh, err := os.Open(f)
		if err != nil {
			fmt.Fprintf(stderr, "ccsed: %s: %v\n", f, err)
			exit = 1
			continue
		}
		readers = append(readers, fh)
		closers = append(closers, fh)
	}
	if len(readers) > 0 {
		if err := eng.Run(io.MultiReader(readers...), stdout); err != nil {
			fmt.Fprintf(stderr, "ccsed: %v\n", err)
			exit = 1
		}
	}
	for _, c := range closers {
		c.Close()
	}
	return exit
}
