// Command ccdbm is a dbm-style key/value database manager.
//
//	ccdbm create test.db
//	ccdbm put test.db greeting "Hello, world!"
//	ccdbm get test.db greeting
package main

import (
	"os"

	"ccdbm/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
