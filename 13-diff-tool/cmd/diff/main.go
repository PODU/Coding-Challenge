// Command diff compares two text files and prints the line edits needed to
// turn the first into the second, in the style of Unix `diff`.
//
// Usage:
//
//	diff [--myers] <file1> <file2>
//
// Output uses "< line" for a line deleted from file1 and "> line" for a line
// added in file2. Exit status: 0 if the files are identical, 1 if they differ,
// 2 on any error (bad arguments, missing or unreadable file).
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/example/diff/internal/diff"
)

const usage = `usage: diff [--myers] <file1> <file2>

Compare two text files and print the line differences.

  < line   line present only in file1 (deleted)
  > line   line present only in file2 (added)

Flags:
  --myers    force the Myers O(ND) algorithm (default: auto-select)
  -h, --help show this help and exit

Exit status: 0 identical, 1 differences found, 2 error.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is main without the os.Exit, so tests can call it.
func run(args []string, stdout, stderr io.Writer) int {
	var (
		forceMyers bool
		paths      []string
	)
	for _, arg := range args {
		switch arg {
		case "-h", "--help":
			fmt.Fprint(stdout, usage)
			return 0
		case "--myers":
			forceMyers = true
		default:
			paths = append(paths, arg)
		}
	}

	if len(paths) != 2 {
		fmt.Fprintf(stderr, "diff: expected exactly 2 file arguments, got %d\n", len(paths))
		fmt.Fprint(stderr, usage)
		return 2
	}

	a, err := readLines(paths[0])
	if err != nil {
		fmt.Fprintf(stderr, "diff: %v\n", err)
		return 2
	}
	b, err := readLines(paths[1])
	if err != nil {
		fmt.Fprintf(stderr, "diff: %v\n", err)
		return 2
	}

	var result []string
	if forceMyers {
		result = diff.DiffMyers(a, b)
	} else {
		result = diff.DiffAuto(a, b)
	}

	if len(result) == 0 {
		return 0
	}
	w := bufio.NewWriter(stdout)
	for _, line := range result {
		fmt.Fprintln(w, line)
	}
	w.Flush()
	return 1
}

// readLines returns the lines of a file with any trailing \r removed, so CRLF
// and LF files compare equal.
func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	// The default 64KiB line limit is too small for some generated files.
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		lines = append(lines, strings.TrimSuffix(sc.Text(), "\r"))
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return lines, nil
}
