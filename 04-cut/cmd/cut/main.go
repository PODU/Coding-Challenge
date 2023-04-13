// Command cut reimplements the Unix cut utility: extract selected portions of
// each input line by byte (-b), character (-c), or delimited field (-f).
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/codingchallenges/cut/internal/cli"
	"github.com/codingchallenges/cut/internal/cut"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run returns the exit status: 0 on success, 1 if any input failed (the
// others are still processed), 2 for a bad command line.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cfg, err := cli.Parse(args)
	if err != nil {
		switch {
		case errors.Is(err, cli.ErrHelp):
			fmt.Fprint(stdout, cli.HelpText)
			return 0
		case errors.Is(err, cli.ErrVersion):
			fmt.Fprintln(stdout, cli.Version)
			return 0
		default:
			fmt.Fprintf(stderr, "cut: %s\n", err)
			fmt.Fprintln(stderr, "Try 'cut --help' for more information.")
			return 2
		}
	}

	cutter := cut.NewCutter(cfg)
	bw := bufio.NewWriter(stdout)
	exit := 0
	for _, name := range cfg.Files {
		if name == "-" {
			if err := cutter.ProcessStream(stdin, bw); err != nil {
				fmt.Fprintf(stderr, "cut: %s: %s\n", displayName(name), err)
				exit = 1
			}
			continue
		}

		f, err := os.Open(name)
		if err != nil {
			fmt.Fprintf(stderr, "cut: %s: %s\n", name, notFoundMessage(err))
			exit = 1
			continue
		}
		if err := cutter.ProcessStream(f, bw); err != nil {
			fmt.Fprintf(stderr, "cut: %s: %s\n", name, err)
			exit = 1
		}
		f.Close()
	}

	if err := bw.Flush(); err != nil {
		fmt.Fprintf(stderr, "cut: write error: %s\n", err)
		return 1
	}
	return exit
}

func displayName(name string) string {
	if name == "-" {
		return "standard input"
	}
	return name
}

// notFoundMessage uses GNU cut's wording instead of the platform's error text,
// which differs on Windows.
func notFoundMessage(err error) string {
	if errors.Is(err, os.ErrNotExist) {
		return "No such file or directory"
	}
	if errors.Is(err, os.ErrPermission) {
		return "Permission denied"
	}
	return err.Error()
}
