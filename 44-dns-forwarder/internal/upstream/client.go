// Package upstream forwards raw DNS queries to a configured recursive resolver
// over UDP, with an optional TCP retry when the UDP response is truncated.
package upstream

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"dns-forwarder/internal/dnsmsg"
)

const udpBufSize = 65535

// Errors surfaced by Exchange.
var (
	ErrShortQuery = errors.New("upstream: query shorter than DNS header")
	ErrTimeout    = errors.New("upstream: timed out waiting for response")
)

// Client is a stateless UDP/TCP forwarder to a single upstream resolver. A
// fresh socket is used per exchange, so concurrent callers never share state
// and responses cannot be cross-correlated by accident.
type Client struct {
	Addr        string        // host:port of the upstream resolver
	Timeout     time.Duration // per-attempt deadline
	Retries     int           // additional attempts after the first on timeout
	TCPFallback bool          // retry over TCP when a UDP response has TC=1
}

// New returns a Client with one retry and TCP fallback disabled.
func New(addr string, timeout time.Duration) *Client {
	return &Client{Addr: addr, Timeout: timeout, Retries: 1}
}

// Exchange sends query to the upstream resolver and returns the raw response.
// It matches the response transaction ID to the query, retries on timeout up to
// Retries times, and (when TCPFallback is set) re-queries over TCP if the UDP
// answer is truncated. The context bounds the total operation.
func (c *Client) Exchange(ctx context.Context, query []byte) ([]byte, error) {
	if len(query) < 12 {
		return nil, ErrShortQuery
	}
	wantID := binary.BigEndian.Uint16(query[0:2])

	attempts := max(c.Retries+1, 1)
	var lastErr error
	for i := 0; i < attempts; i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		resp, err := c.exchangeUDP(ctx, query, wantID)
		if err != nil {
			lastErr = err
			continue
		}
		if c.TCPFallback && dnsmsg.Truncated(resp) {
			if tcpResp, terr := c.exchangeTCP(ctx, query); terr == nil {
				return tcpResp, nil
			}
			// TCP failed: hand back the truncated UDP answer and let the
			// client retry over TCP itself.
		}
		return resp, nil
	}
	if lastErr == nil {
		lastErr = ErrTimeout
	}
	return nil, lastErr
}

func (c *Client) deadline(ctx context.Context) time.Time {
	dl := time.Now().Add(c.Timeout)
	if ctxDL, ok := ctx.Deadline(); ok && ctxDL.Before(dl) {
		return ctxDL
	}
	return dl
}

func (c *Client) exchangeUDP(ctx context.Context, query []byte, wantID uint16) ([]byte, error) {
	conn, err := net.Dial("udp", c.Addr)
	if err != nil {
		return nil, fmt.Errorf("upstream: dial: %w", err)
	}
	defer conn.Close()

	deadline := c.deadline(ctx)
	if err := conn.SetDeadline(deadline); err != nil {
		return nil, err
	}
	if _, err := conn.Write(query); err != nil {
		return nil, fmt.Errorf("upstream: write: %w", err)
	}

	buf := make([]byte, udpBufSize)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				return nil, ErrTimeout
			}
			return nil, fmt.Errorf("upstream: read: %w", err)
		}
		if n < 12 {
			continue // too short to be DNS; keep waiting
		}
		if binary.BigEndian.Uint16(buf[0:2]) != wantID {
			continue // late or spoofed reply to some other query
		}
		return append([]byte(nil), buf[:n]...), nil
	}
}

func (c *Client) exchangeTCP(ctx context.Context, query []byte) ([]byte, error) {
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "tcp", c.Addr)
	if err != nil {
		return nil, fmt.Errorf("upstream: tcp dial: %w", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(c.deadline(ctx)); err != nil {
		return nil, err
	}

	// DNS over TCP prefixes each message with a 2-byte length.
	framed := make([]byte, 2+len(query))
	binary.BigEndian.PutUint16(framed[0:2], uint16(len(query)))
	copy(framed[2:], query)
	if _, err := conn.Write(framed); err != nil {
		return nil, fmt.Errorf("upstream: tcp write: %w", err)
	}

	var lenBuf [2]byte
	if _, err := io.ReadFull(conn, lenBuf[:]); err != nil {
		return nil, fmt.Errorf("upstream: tcp read len: %w", err)
	}
	msgLen := int(binary.BigEndian.Uint16(lenBuf[:]))
	msg := make([]byte, msgLen)
	if _, err := io.ReadFull(conn, msg); err != nil {
		return nil, fmt.Errorf("upstream: tcp read body: %w", err)
	}
	return msg, nil
}
