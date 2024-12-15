package endpoint

import (
	"context"
	"fmt"
	"net"

	"ccsocat/internal/addr"
)

// Listener hands out one Endpoint per peer. A UDP listener only ever accepts
// one peer.
type Listener interface {
	Accept() (Endpoint, error)
	Close() error // also unblocks a pending Accept
	Addr() net.Addr
}

// Listen opens a TCP-LISTEN or UDP-LISTEN address.
func Listen(spec *addr.AddressSpec) (Listener, error) {
	switch spec.Kind {
	case addr.KindTCPListen:
		return listenTCP(spec)
	case addr.KindUDPListen:
		return listenUDP(spec)
	default:
		return nil, fmt.Errorf("%s is not a listening address", spec.Kind)
	}
}

func listenTCP(spec *addr.AddressSpec) (Listener, error) {
	lc := net.ListenConfig{}
	if spec.HasOption("reuseaddr") {
		lc.Control = controlReuseAddr
	}
	ln, err := lc.Listen(context.Background(), spec.Net, ":"+spec.Port())
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", spec.Raw, err)
	}
	return &tcpListener{ln: ln}, nil
}

type tcpListener struct{ ln net.Listener }

func (t *tcpListener) Accept() (Endpoint, error) { return t.ln.Accept() }
func (t *tcpListener) Close() error              { return t.ln.Close() }
func (t *tcpListener) Addr() net.Addr            { return t.ln.Addr() }

// udpListener has no connections to accept, so it treats the first peer to
// send a datagram as the connection and ignores everyone else.
type udpListener struct {
	conn *net.UDPConn
	done bool
}

func listenUDP(spec *addr.AddressSpec) (Listener, error) {
	laddr, err := net.ResolveUDPAddr(spec.Net, ":"+spec.Port())
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", spec.Raw, err)
	}
	conn, err := net.ListenUDP(spec.Net, laddr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", spec.Raw, err)
	}
	return &udpListener{conn: conn}, nil
}

func (u *udpListener) Accept() (Endpoint, error) {
	if u.done {
		return nil, fmt.Errorf("udp listener already accepted its peer")
	}
	buf := make([]byte, 64*1024)
	n, peer, err := u.conn.ReadFromUDP(buf)
	if err != nil {
		return nil, err
	}
	u.done = true
	return &udpServerConn{
		conn:  u.conn,
		peer:  peer,
		first: append([]byte(nil), buf[:n]...),
	}, nil
}

func (u *udpListener) Close() error   { return u.conn.Close() }
func (u *udpListener) Addr() net.Addr { return u.conn.LocalAddr() }

// udpServerConn turns the listening socket into a stream with the chosen
// peer. The datagram consumed by Accept is returned by the first Read.
type udpServerConn struct {
	conn  *net.UDPConn
	peer  *net.UDPAddr
	first []byte
}

func (c *udpServerConn) Read(p []byte) (int, error) {
	if len(c.first) > 0 {
		n := copy(p, c.first)
		c.first = c.first[n:]
		return n, nil
	}
	n, _, err := c.conn.ReadFromUDP(p)
	return n, err
}

func (c *udpServerConn) Write(p []byte) (int, error) {
	return c.conn.WriteToUDP(p, c.peer)
}

func (c *udpServerConn) Close() error { return c.conn.Close() }
