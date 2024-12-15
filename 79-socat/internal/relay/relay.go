// Package relay copies bytes between two endpoints.
package relay

import (
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"sync"
)

// Mode selects which way data flows.
type Mode int

const (
	Bidirectional  Mode = iota
	UniLeftToRight      // -u
	UniRightToLeft      // -U
)

// HalfCloser is implemented by endpoints that can shut one direction while
// the other keeps going, like *net.TCPConn. Endpoints without it are only
// closed once both directions finish.
type HalfCloser interface {
	CloseRead() error
	CloseWrite() error
}

// Relay copies between left and right and closes both before returning.
// Ordinary EOF or close is not an error.
func Relay(left, right io.ReadWriteCloser, mode Mode) error {
	switch mode {
	case UniLeftToRight:
		return uni(left, right)
	case UniRightToLeft:
		return uni(right, left)
	default:
		return bidi(left, right)
	}
}

func uni(src, dst io.ReadWriteCloser) error {
	_, err := io.Copy(dst, src)
	closeWrite(dst)
	closeRead(src)
	dstErr := dst.Close()
	srcErr := src.Close()
	return firstError(err, dstErr, srcErr)
}

// bidi copies in both directions. When one direction hits EOF it is shut
// down on its own where the endpoints allow it, so the peer sees the EOF
// while the other direction carries on.
func bidi(left, right io.ReadWriteCloser) error {
	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, errs[0] = io.Copy(right, left)
		closeWrite(right)
		closeRead(left)
	}()
	go func() {
		defer wg.Done()
		_, errs[1] = io.Copy(left, right)
		closeWrite(left)
		closeRead(right)
	}()

	wg.Wait()
	leftErr := left.Close()
	rightErr := right.Close()
	return firstError(errs[0], errs[1], leftErr, rightErr)
}

func closeWrite(c io.ReadWriteCloser) {
	if hc, ok := c.(HalfCloser); ok {
		_ = hc.CloseWrite()
	}
}

func closeRead(c io.ReadWriteCloser) {
	if hc, ok := c.(HalfCloser); ok {
		_ = hc.CloseRead()
	}
}

// firstError returns the first error that isn't just a side effect of
// shutting down.
func firstError(errs ...error) error {
	for _, err := range errs {
		if !benign(err) {
			return err
		}
	}
	return nil
}

// benign reports whether err is EOF or a "use of closed connection" error,
// which the two copy goroutines routinely cause each other while closing.
func benign(err error) bool {
	if err == nil || err == io.EOF {
		return true
	}
	if errors.Is(err, net.ErrClosed) ||
		errors.Is(err, io.ErrClosedPipe) ||
		errors.Is(err, os.ErrClosed) {
		return true
	}
	// Some closed-handle errors from the stdlib aren't wrapped sentinels.
	return strings.Contains(err.Error(), "closed")
}
