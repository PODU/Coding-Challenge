// Package tracer runs the traced process and reports each syscall it makes.
// The ptrace loop is in tracer_linux.go; on other systems New just returns
// ErrTracingUnsupported.
package tracer

import "errors"

// RawCall is one syscall as read from the registers. It is plain data apart
// from Mem, so tests can build them by hand.
type RawCall struct {
	Nr      int       // syscall number (ORIG_RAX on x86-64)
	Args    [6]uint64 // raw register arguments (RDI, RSI, RDX, R10, R8, R9)
	Ret     int64     // return value, valid only when HasRet is true (RAX)
	HasRet  bool      // true on syscall-exit; false for no-return calls
	Mem     MemReader // for reading string arguments
	StartNs int64     // UnixNano at syscall entry
	DurNs   int64     // time from entry to exit
}

// Tracer runs a trace, calling handler once per syscall.
type Tracer interface {
	// Run traces until the process exits and returns its exit code.
	Run(handler func(RawCall)) (exitCode int, err error)
}

// ErrTracingUnsupported is returned by New on non-Linux hosts.
var ErrTracingUnsupported = errors.New("live tracing requires Linux")
