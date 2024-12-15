//go:build !windows

package endpoint

import (
	"syscall"
)

// controlReuseAddr sets SO_REUSEADDR before bind.
func controlReuseAddr(network, address string, c syscall.RawConn) error {
	var serr error
	if err := c.Control(func(fd uintptr) {
		serr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	}); err != nil {
		return err
	}
	return serr
}
