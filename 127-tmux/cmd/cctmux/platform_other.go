//go:build !linux

package main

import "fmt"

// The client and server need PTYs and Unix sockets, so elsewhere we just say so.
func platformRun(cfg Config, req request) error {
	return fmt.Errorf("the interactive server/client requires Linux; " +
		"run `cctmux demo` to see the engine, or `go test ./...` for the full suite")
}
