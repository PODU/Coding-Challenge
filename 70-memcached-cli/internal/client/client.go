// Package client sends memcached requests over TCP. Every operation has a
// deadline so a dead server can't hang the CLI.
package client

import (
	"bufio"
	"net"
	"time"
)

// Client is one connection to a memcached server. It is not safe for
// concurrent use.
type Client struct {
	conn    net.Conn
	r       *bufio.Reader
	w       *bufio.Writer
	timeout time.Duration
	addr    string
}

// Dial connects to host:port. timeout applies to the dial and to each
// operation; zero or less means no timeout.
func Dial(host, port string, timeout time.Duration) (*Client, error) {
	addr := net.JoinHostPort(host, port)
	var (
		conn net.Conn
		err  error
	)
	if timeout > 0 {
		conn, err = net.DialTimeout("tcp", addr, timeout)
	} else {
		conn, err = net.Dial("tcp", addr)
	}
	if err != nil {
		return nil, err
	}
	return newClient(conn, addr, timeout), nil
}

func newClient(conn net.Conn, addr string, timeout time.Duration) *Client {
	return &Client{
		conn:    conn,
		r:       bufio.NewReader(conn),
		w:       bufio.NewWriter(conn),
		timeout: timeout,
		addr:    addr,
	}
}

// Addr returns the dialed host:port.
func (c *Client) Addr() string { return c.addr }

// Close closes the connection.
func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// armDeadline sets the deadline for the next operation.
func (c *Client) armDeadline() error {
	if c.timeout <= 0 {
		return c.conn.SetDeadline(time.Time{})
	}
	return c.conn.SetDeadline(time.Now().Add(c.timeout))
}

func (c *Client) send(b []byte) error {
	if _, err := c.w.Write(b); err != nil {
		return err
	}
	return c.w.Flush()
}
