// Package transport sends a DNS query to a server and returns the raw reply.
package transport

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"time"
)

// Exchanger sends one query and returns the reply. Tests use a fake one.
type Exchanger interface {
	Exchange(server netip.AddrPort, query []byte) ([]byte, error)
}

var ErrTimeout = errors.New("transport: query timed out")

// UDP is an Exchanger that queries over UDP, retrying on errors, and switches
// to TCP when the reply is truncated. Zero fields take the defaults below.
type UDP struct {
	Timeout    time.Duration // per attempt
	Retries    int           // UDP attempts per query
	ForceTCP   bool          // skip UDP altogether
	MaxUDPSize int           // receive buffer size
}

const (
	defaultTimeout = 3 * time.Second
	defaultRetries = 2
	defaultUDPSize = 4096
)

func (u UDP) Exchange(server netip.AddrPort, query []byte) ([]byte, error) {
	timeout := u.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	retries := u.Retries
	if retries < 1 {
		retries = defaultRetries
	}

	if u.ForceTCP {
		return u.exchangeTCP(server, query, timeout)
	}

	var lastErr error
	for attempt := 0; attempt < retries; attempt++ {
		resp, err := u.exchangeUDPOnce(server, query, timeout)
		if err != nil {
			lastErr = err
			continue
		}
		if isTruncated(resp) {
			return u.exchangeTCP(server, query, timeout)
		}
		return resp, nil
	}
	if lastErr == nil {
		lastErr = ErrTimeout
	}
	return nil, fmt.Errorf("udp %s: %w", server, lastErr)
}

func (u UDP) exchangeUDPOnce(server netip.AddrPort, query []byte, timeout time.Duration) ([]byte, error) {
	conn, err := net.DialUDP("udp", nil, net.UDPAddrFromAddrPort(server))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}
	if _, err := conn.Write(query); err != nil {
		return nil, err
	}

	size := u.MaxUDPSize
	if size <= 0 {
		size = defaultUDPSize
	}
	buf := make([]byte, size)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}
	out := make([]byte, n)
	copy(out, buf[:n])
	return out, nil
}

func (u UDP) exchangeTCP(server netip.AddrPort, query []byte, timeout time.Duration) ([]byte, error) {
	conn, err := net.DialTimeout("tcp", server.String(), timeout)
	if err != nil {
		return nil, fmt.Errorf("tcp %s: %w", server, err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}

	// Over TCP each message is preceded by its length as two bytes.
	framed := make([]byte, 2+len(query))
	binary.BigEndian.PutUint16(framed[0:2], uint16(len(query)))
	copy(framed[2:], query)
	if _, err := conn.Write(framed); err != nil {
		return nil, fmt.Errorf("tcp %s write: %w", server, err)
	}

	var lenBuf [2]byte
	if _, err := io.ReadFull(conn, lenBuf[:]); err != nil {
		return nil, fmt.Errorf("tcp %s read length: %w", server, err)
	}
	respLen := binary.BigEndian.Uint16(lenBuf[:])
	resp := make([]byte, respLen)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return nil, fmt.Errorf("tcp %s read body: %w", server, err)
	}
	return resp, nil
}

// isTruncated reports whether the TC flag is set. Anything too short to have
// one counts as not truncated.
func isTruncated(resp []byte) bool {
	return len(resp) >= 4 && binary.BigEndian.Uint16(resp[2:4])&(1<<9) != 0
}
