// Command comm compares two sorted files line by line and prints three
// tab-separated columns: lines only in FILE1, lines only in FILE2, and lines
// common to both.
package main

import (
	"os"

	"commclone/internal/comm"
)

func main() {
	os.Exit(comm.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
