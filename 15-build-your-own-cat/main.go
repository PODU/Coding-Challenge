// Command ccat is a clone of the Unix `cat` utility: it concatenates files
// (or standard input) to standard output, with optional line numbering.
package main

import "os"

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
