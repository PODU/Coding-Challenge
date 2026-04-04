// Command gzipcc is a gzip/gunzip clone with its own DEFLATE implementation.
// The standard library's compress packages are only used by the tests, to
// cross-check the output.
package main

import (
	"os"

	"gzipcc/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
