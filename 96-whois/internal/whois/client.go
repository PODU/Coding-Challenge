package whois

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

// DefaultPort is the standard WHOIS port.
const DefaultPort = 43

// DefaultTimeout limits each server query, connect and read together.
const DefaultTimeout = 10 * time.Second

// Dialer is satisfied by *net.Dialer. Tests use it to connect to a local
// fake server.
type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// Client speaks the WHOIS protocol itself: send one line, read until the
// server closes the connection.
type Client struct {
	Dialer  Dialer        // nil means a plain *net.Dialer
	Port    int           // used when the server name has no port
	Timeout time.Duration // per query
}

// NewClient returns a Client with the default port and timeout.
func NewClient() *Client {
	return &Client{
		Dialer:  &net.Dialer{},
		Port:    DefaultPort,
		Timeout: DefaultTimeout,
	}
}

// address adds the client's port to server unless it already has one.
func (c *Client) address(server string) string {
	if _, _, err := net.SplitHostPort(server); err == nil {
		return server
	}
	port := c.Port
	if port == 0 {
		port = DefaultPort
	}
	return net.JoinHostPort(server, strconv.Itoa(port))
}

// Query sends request to server and returns the raw response. Query adds the
// CRLF itself.
func (c *Client) Query(ctx context.Context, server, request string) (string, error) {
	timeout := c.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	d := c.Dialer
	if d == nil {
		d = &net.Dialer{}
	}
	address := c.address(server)

	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn, err := d.DialContext(dialCtx, "tcp", address)
	if err != nil {
		return "", fmt.Errorf("whois: dial %s: %w", address, err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	if _, err := io.WriteString(conn, request+"\r\n"); err != nil {
		return "", fmt.Errorf("whois: write %s: %w", address, err)
	}

	data, err := io.ReadAll(conn)
	if err != nil {
		return "", fmt.Errorf("whois: read %s: %w", address, err)
	}
	return string(data), nil
}
