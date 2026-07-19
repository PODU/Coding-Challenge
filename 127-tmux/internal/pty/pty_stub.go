//go:build !linux

package pty

import "errors"

// OpenShell is only implemented on Linux. Elsewhere, use NewMock.
func OpenShell(shell string, w, h int) (PTY, error) {
	return nil, errors.New("pty: real shells require Linux; use the mock/demo on this platform")
}
