// Package ccecho implements an RFC 862 echo server supporting TCP and UDP.
package ccecho

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"time"
)

const (
	// DefaultPort is the RFC 862 echo port.
	DefaultPort         = 7
	DefaultDrainTimeout = 10 * time.Second
	// MaxUDPDatagram is the largest IPv4 UDP payload, so one ReadFrom never
	// truncates a datagram.
	MaxUDPDatagram = 65507
)

// Config configures a Server.
type Config struct {
	Host     string // "" binds all interfaces
	Port     int
	Protocol string // "tcp" or "udp"
	// IdleTimeout is the per-connection read deadline. 0 disables it.
	IdleTimeout time.Duration
	// DrainTimeout limits how long shutdown waits for open connections.
	// 0 means wait forever.
	DrainTimeout time.Duration
	Logger       *slog.Logger // defaults to slog.Default()
}

func (c *Config) validate() error {
	// Port 0 lets the OS pick a free port, which the tests rely on.
	if c.Port < 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port %d: must be in range 0..65535", c.Port)
	}
	switch c.Protocol {
	case "tcp", "udp":
	case "":
		c.Protocol = "tcp"
	default:
		return fmt.Errorf("invalid protocol %q: must be \"tcp\" or \"udp\"", c.Protocol)
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	return nil
}

func (c *Config) address() string {
	return net.JoinHostPort(c.Host, fmt.Sprintf("%d", c.Port))
}

// Server is an echo server for a single protocol.
type Server struct {
	cfg Config
	log *slog.Logger

	wg sync.WaitGroup

	mu       sync.Mutex
	tcpLn    *net.TCPListener
	udpConn  *net.UDPConn
	closed   bool // listener closed for shutdown
	addrOnce sync.Once
	addrCh   chan net.Addr
}

// NewServer validates cfg and returns a Server ready to Run.
func NewServer(cfg Config) (*Server, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &Server{
		cfg:    cfg,
		log:    cfg.Logger,
		addrCh: make(chan net.Addr, 1),
	}, nil
}

// Addr blocks until the server is listening and returns its address, or nil
// if ctx is done first. Useful when binding to port 0.
func (s *Server) Addr(ctx context.Context) net.Addr {
	select {
	case addr := <-s.addrCh:
		// Put it back so later callers see it too.
		select {
		case s.addrCh <- addr:
		default:
		}
		return addr
	case <-ctx.Done():
		return nil
	}
}

func (s *Server) publishAddr(addr net.Addr) {
	s.addrOnce.Do(func() {
		select {
		case s.addrCh <- addr:
		default:
		}
	})
}

// Run serves until ctx is cancelled or a fatal error occurs. On cancellation
// it stops accepting and waits up to DrainTimeout for open connections.
func (s *Server) Run(ctx context.Context) error {
	switch s.cfg.Protocol {
	case "tcp":
		return s.serveTCP(ctx)
	case "udp":
		return s.serveUDP(ctx)
	default:
		return fmt.Errorf("unsupported protocol %q", s.cfg.Protocol)
	}
}

func (s *Server) serveTCP(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.cfg.address())
	if err != nil {
		return fmt.Errorf("listen tcp %s: %w", s.cfg.address(), err)
	}
	tcpLn := ln.(*net.TCPListener)

	s.mu.Lock()
	s.tcpLn = tcpLn
	s.mu.Unlock()

	s.publishAddr(tcpLn.Addr())
	s.log.Info("echo server listening", "protocol", "tcp", "addr", tcpLn.Addr().String())

	// Closing the listener on cancellation unblocks Accept.
	stop := context.AfterFunc(ctx, func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		_ = tcpLn.Close()
	})
	defer stop()

	for {
		conn, err := tcpLn.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				break
			}
			if ne, ok := err.(net.Error); ok && ne.Temporary() { //nolint:staticcheck // Temporary still useful here
				s.log.Warn("temporary accept error", "err", err)
				continue
			}
			return fmt.Errorf("accept: %w", err)
		}
		s.wg.Add(1)
		go s.handleConn(conn)
	}

	s.log.Info("tcp accept loop stopped; draining active connections")
	return s.drain()
}

// drain waits for connection goroutines to finish, giving up after
// DrainTimeout.
func (s *Server) drain() error {
	if s.cfg.DrainTimeout <= 0 {
		s.wg.Wait()
		s.log.Info("all connections drained")
		return nil
	}

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	timer := time.NewTimer(s.cfg.DrainTimeout)
	defer timer.Stop()

	select {
	case <-done:
		s.log.Info("all connections drained")
		return nil
	case <-timer.C:
		s.log.Warn("drain timeout elapsed; some connections may have been force-closed",
			"timeout", s.cfg.DrainTimeout)
		return nil
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	remote := conn.RemoteAddr().String()
	s.log.Info("connection accepted", "remote", remote)

	n, err := io.Copy(conn, &idleReader{conn: conn, timeout: s.cfg.IdleTimeout})
	if err == nil || errors.Is(err, io.EOF) {
		s.log.Info("connection closed", "remote", remote, "bytes", n)
	} else {
		s.log.Debug("connection ended with error", "remote", remote, "bytes", n, "err", err)
	}
}

// idleReader refreshes the read deadline before each Read so a stalled peer
// can't hold a connection (and shutdown) open forever.
type idleReader struct {
	conn    net.Conn
	timeout time.Duration
}

func (r *idleReader) Read(p []byte) (int, error) {
	if r.timeout > 0 {
		_ = r.conn.SetReadDeadline(time.Now().Add(r.timeout))
	}
	return r.conn.Read(p)
}

// serveUDP echoes each datagram back to its sender from a single read loop.
func (s *Server) serveUDP(ctx context.Context) error {
	addr, err := net.ResolveUDPAddr("udp", s.cfg.address())
	if err != nil {
		return fmt.Errorf("resolve udp %s: %w", s.cfg.address(), err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("listen udp %s: %w", s.cfg.address(), err)
	}

	s.mu.Lock()
	s.udpConn = conn
	s.mu.Unlock()

	s.publishAddr(conn.LocalAddr())
	s.log.Info("echo server listening", "protocol", "udp", "addr", conn.LocalAddr().String())

	stop := context.AfterFunc(ctx, func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		_ = conn.Close()
	})
	defer stop()

	buf := make([]byte, MaxUDPDatagram)
	for {
		n, src, err := conn.ReadFrom(buf)
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				s.log.Info("udp read loop stopped")
				return nil
			}
			if ne, ok := err.(net.Error); ok && ne.Temporary() { //nolint:staticcheck // Temporary still useful here
				s.log.Warn("temporary udp read error", "err", err)
				continue
			}
			return fmt.Errorf("udp read: %w", err)
		}
		// Zero-length datagrams are echoed too.
		if _, err := conn.WriteTo(buf[:n], src); err != nil {
			s.log.Debug("udp write error", "remote", src.String(), "err", err)
		}
	}
}
