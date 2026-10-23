//go:build !unix && !windows

package main

import "os"

func openShared(path string) (*os.File, error) { return os.Open(path) }

// processAlive cannot be determined here, so --pid never ends following.
func processAlive(int) bool { return true }
