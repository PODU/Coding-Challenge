// Package conn reads and writes IRC lines over a net.Conn. Sends are
// serialised so two goroutines can't interleave their lines on the wire.
package conn

import (
	"bufio"
	"context"
	"crypto/tls"
	"net"
	"strings"
	"sync"
	"time"
)

// maxLine is IRC's message length limit, CRLF included. Longer outgoing lines
// are truncated.
const maxLine = 512

// Conn is a line-framed IRC connection.
type Conn struct {
	nc    net.Conn
	w     *bufio.Writer
	lines chan string
	errs  chan error

	writeMu   sync.Mutex
	closeOnce sync.Once
	closed    chan struct{}
}

// Dial connects to addr ("host:port"), over TLS if asked.
func Dial(ctx context.Context, addr string, useTLS bool) (*Conn, error) {
	d := net.Dialer{Timeout: 30 * time.Second}
	var nc net.Conn
	var err error
	if useTLS {
		td := &tls.Dialer{NetDialer: &d}
		nc, err = td.DialContext(ctx, "tcp", addr)
	} else {
		nc, err = d.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return nil, err
	}
	return New(nc), nil
}

// New wraps an already open connection, such as one end of a net.Pipe.
func New(nc net.Conn) *Conn {
	c := &Conn{
		nc:     nc,
		w:      bufio.NewWriter(nc),
		lines:  make(chan string, 64),
		errs:   make(chan error, 1),
		closed: make(chan struct{}),
	}
	go c.readLoop()
	return c
}

func (c *Conn) readLoop() {
	defer close(c.lines)
	r := bufio.NewReader(c.nc)
	for {
		line, err := r.ReadString('\n')
		// A final line without a newline is still delivered.
		if trimmed := strings.TrimRight(line, "\r\n"); trimmed != "" {
			select {
			case c.lines <- trimmed:
			case <-c.closed:
				return
			}
		}
		if err != nil {
			select {
			case c.errs <- err:
			default:
			}
			return
		}
	}
}

// Send writes one line, terminated with CRLF and cut to the length limit. It
// is safe to call from several goroutines.
func (c *Conn) Send(line string) error {
	line = strings.TrimRight(line, "\r\n")
	if len(line) > maxLine-2 {
		line = line[:maxLine-2]
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if _, err := c.w.WriteString(line + "\r\n"); err != nil {
		return err
	}
	return c.w.Flush()
}

// Lines delivers incoming lines without their CRLF. It is closed when reading
// stops.
func (c *Conn) Lines() <-chan string { return c.lines }

// Errs returns a channel that receives the first fatal read error.
func (c *Conn) Errs() <-chan error { return c.errs }

// Close closes the connection. Calling it again is a no-op.
func (c *Conn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		close(c.closed)
		err = c.nc.Close()
	})
	return err
}
