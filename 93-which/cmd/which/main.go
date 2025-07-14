// Command which finds programs on PATH, like BSD which. -a prints every
// match and -s prints nothing, leaving just the exit status:
//
//	0  every name was found
//	1  at least one name was not found
//	2  usage error
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/codingchallenges/which/internal/resolver"
)

const usage = `usage: which [-as] program ...`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr,
		os.Getenv("PATH"), os.Getenv("PATHEXT")))
}

// run takes PATH and PATHEXT as arguments so tests don't have to change the
// real environment.
func run(args []string, stdout, stderr io.Writer, pathEnv, pathextEnv string) int {
	fs := flag.NewFlagSet("which", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprintln(stderr, usage) }

	var (
		all    bool
		silent bool
	)
	fs.BoolVar(&all, "a", false, "list all matches in PATH, not just the first")
	fs.BoolVar(&all, "all", false, "list all matches in PATH, not just the first")
	fs.BoolVar(&silent, "s", false, "silent; no output, exit status only")
	fs.BoolVar(&silent, "silent", false, "silent; no output, exit status only")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	names := fs.Args()
	if len(names) == 0 {
		fmt.Fprintln(stderr, usage)
		return 2
	}

	dirs := resolver.SplitPath(pathEnv)
	exts := resolver.SplitExts(pathextEnv)

	allFound := true
	for _, name := range names {
		matches := resolver.Resolve(name, dirs, exts, all)
		if len(matches) == 0 {
			allFound = false
			if !silent {
				fmt.Fprintf(stderr, "which: no %s in (%s)\n", name, pathEnv)
			}
			continue
		}
		if !silent {
			for _, m := range matches {
				fmt.Fprintln(stdout, m)
			}
		}
	}

	if !allFound {
		return 1
	}
	return 0
}
