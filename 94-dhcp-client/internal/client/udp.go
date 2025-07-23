package client

import (
	"fmt"
	"net"
	"syscall"
	"time"
)

const (
	ClientPort = 68
	ServerPort = 67
)

var broadcastAddr = &net.UDPAddr{IP: net.IPv4bcast, Port: ServerPort}

// UDPTransport listens on UDP :68 and broadcasts to 255.255.255.255:67. It
// needs admin/root.
type UDPTransport struct {
	conn *net.UDPConn
}

// NewUDPTransport binds the client port on all interfaces. Without enough
// privilege this fails with a permission error.
func NewUDPTransport() (*UDPTransport, error) {
	lc := net.ListenConfig{
		Control: func(_, _ string, c syscall.RawConn) error {
			var sockErr error
			if err := c.Control(func(fd uintptr) {
				sockErr = setSocketOptions(fd)
			}); err != nil {
				return err
			}
			return sockErr
		},
	}

	pc, err := lc.ListenPacket(nil, "udp4", fmt.Sprintf(":%d", ClientPort))
	if err != nil {
		return nil, fmt.Errorf("bind udp :%d (needs admin/root): %w", ClientPort, err)
	}
	conn, ok := pc.(*net.UDPConn)
	if !ok {
		_ = pc.Close()
		return nil, fmt.Errorf("unexpected packet conn type %T", pc)
	}
	return &UDPTransport{conn: conn}, nil
}

func (t *UDPTransport) Send(b []byte) error {
	if _, err := t.conn.WriteToUDP(b, broadcastAddr); err != nil {
		return fmt.Errorf("broadcast: %w", err)
	}
	return nil
}

func (t *UDPTransport) Receive(deadline time.Time) ([]byte, error) {
	if err := t.conn.SetReadDeadline(deadline); err != nil {
		return nil, err
	}
	buf := make([]byte, 1500)
	n, _, err := t.conn.ReadFromUDP(buf)
	if err != nil {
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			return nil, ErrTimeout
		}
		return nil, err
	}
	return buf[:n], nil
}

func (t *UDPTransport) Close() error {
	if t.conn == nil {
		return nil
	}
	return t.conn.Close()
}
