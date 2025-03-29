//go:build !linux

package tracer

import "mystrace/internal/cli"

// New always returns ErrTracingUnsupported outside Linux.
func New(_ cli.Config) (Tracer, error) {
	return nil, ErrTracingUnsupported
}
