//go:build !unix

package main

import "io"

// setupSignalHandler does nothing on platforms without SIGUSR1, such as Windows.
func setupSignalHandler(_ *Stats, _ io.Writer) func() {
	return func() {}
}
