// Command md5sum is a clone of the Unix md5sum utility. The digest itself is
// computed by internal/md5; crypto/md5 is only used by the tests to check it.
package main

import (
	"os"

	"md5sum/internal/app"
)

func main() {
	os.Exit(app.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
