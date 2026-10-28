package protocol

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// MaxMessageSize is Barrier's PROTOCOL_MAX_MESSAGE_LENGTH. Each message on
// the wire is a 4-byte big-endian length followed by that many bytes.
const MaxMessageSize = 4 * 1024 * 1024

// WriteFrame writes payload with its length prefix in a single Write, so
// frames from concurrent writers never interleave on the socket.
func WriteFrame(w io.Writer, payload []byte) error {
	if len(payload) > MaxMessageSize {
		return fmt.Errorf("protocol: message of %d bytes exceeds limit", len(payload))
	}
	buf := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(buf, uint32(len(payload)))
	copy(buf[4:], payload)
	_, err := w.Write(buf)
	return err
}

// ReadFrame reads one length-prefixed payload.
func ReadFrame(r io.Reader) ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n > MaxMessageSize {
		return nil, fmt.Errorf("protocol: peer announced a %d-byte message", n)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}
	return buf, nil
}

// Conn is a message-level connection. Send is safe for concurrent use;
// Recv must be called from one goroutine.
type Conn struct {
	c            net.Conn
	r            *bufio.Reader
	mu           sync.Mutex
	WriteTimeout time.Duration
}

// NewConn wraps a network connection.
func NewConn(c net.Conn) *Conn {
	return &Conn{c: c, r: bufio.NewReader(c), WriteTimeout: 5 * time.Second}
}

// Send writes one message. A peer that stops reading cannot stall the caller
// for longer than WriteTimeout.
func (c *Conn) Send(m Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.WriteTimeout > 0 {
		c.c.SetWriteDeadline(time.Now().Add(c.WriteTimeout))
	}
	return WriteFrame(c.c, m.Marshal())
}

// Recv reads and decodes one message. If timeout is positive the read fails
// once that much time passes without a complete message.
func (c *Conn) Recv(timeout time.Duration) (Message, error) {
	if timeout > 0 {
		c.c.SetReadDeadline(time.Now().Add(timeout))
	} else {
		c.c.SetReadDeadline(time.Time{})
	}
	b, err := ReadFrame(c.r)
	if err != nil {
		return nil, err
	}
	return Parse(b)
}

// Close closes the underlying connection.
func (c *Conn) Close() error { return c.c.Close() }

// RemoteAddr is the peer's address.
func (c *Conn) RemoteAddr() net.Addr { return c.c.RemoteAddr() }
