//go:build unix

package main

import (
	"os"
	"syscall"
)

// openShared opens path for reading. On Unix an open file can always be
// renamed or unlinked by others, so plain os.Open suffices.
func openShared(path string) (*os.File, error) {
	return os.Open(path)
}

// processAlive reports whether process pid is still running. Signal 0 checks
// existence without delivering anything; EPERM means it exists but is owned
// by another user.
func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
