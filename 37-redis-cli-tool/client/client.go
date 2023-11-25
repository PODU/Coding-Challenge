// Package client sends commands to a Redis server and reads the replies.
package client

import (
	"bufio"
	"net"

	"rediscli/resp"
)

type Client struct {
	conn net.Conn
	r    *bufio.Reader
	w    *bufio.Writer
	addr string
}

func Dial(host, port string) (*Client, error) {
	addr := net.JoinHostPort(host, port)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return New(conn, addr), nil
}

// New wraps an existing connection, such as one end of a net.Pipe in tests.
func New(conn net.Conn, addr string) *Client {
	return &Client{
		conn: conn,
		r:    bufio.NewReader(conn),
		w:    bufio.NewWriter(conn),
		addr: addr,
	}
}

// Send sends one command and waits for its reply.
func (c *Client) Send(args []string) (resp.Value, error) {
	if _, err := c.w.Write(resp.Encode(args)); err != nil {
		return resp.Value{}, err
	}
	if err := c.w.Flush(); err != nil {
		return resp.Value{}, err
	}
	return resp.Decode(c.r)
}

func (c *Client) Addr() string { return c.addr }

func (c *Client) Close() error { return c.conn.Close() }
