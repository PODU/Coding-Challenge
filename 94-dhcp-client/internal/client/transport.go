// Package client runs the DHCP DORA exchange over a Transport. The clock and
// random source are injectable so tests can use an in-memory fake server.
package client

import (
	"errors"
	"time"
)

// ErrTimeout is what Transport.Receive returns when the deadline passes. The
// client retransmits when it sees it.
var ErrTimeout = errors.New("dhcp: receive timeout")

// Transport sends DHCP packets and receives replies.
type Transport interface {
	Send(b []byte) error
	Receive(deadline time.Time) ([]byte, error)
	Close() error
}
