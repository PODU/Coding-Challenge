package smtp

import (
	"context"
	"net"
	"os"
	"sync"
	"time"
)

// Server is an SMTP server that handles each connection in its own goroutine.
type Server struct {
	Hostname string // announced in the greeting and EHLO reply
	Store    MessageStore
	Timeout  time.Duration // per-connection idle deadline

	listener net.Listener
	wg       sync.WaitGroup

	mu     sync.Mutex
	closed bool
}

// NewServer returns a Server. An empty hostname means the OS hostname and a
// zero timeout means five minutes.
func NewServer(hostname string, store MessageStore, timeout time.Duration) *Server {
	if hostname == "" {
		if h, err := os.Hostname(); err == nil && h != "" {
			hostname = h
		} else {
			hostname = "localhost"
		}
	}
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	return &Server{Hostname: hostname, Store: store, Timeout: timeout}
}

// Listen binds addr. It is separate from Serve so callers (tests using ":0")
// can read Addr before serving.
func (s *Server) Listen(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.listener = ln
	return nil
}

// Addr returns the bound address, or nil before Listen.
func (s *Server) Addr() net.Addr {
	if s.listener == nil {
		return nil
	}
	return s.listener.Addr()
}

// ListenAndServe binds addr if needed and serves until ctx is canceled or
// Close is called.
func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	if s.listener == nil {
		if err := s.Listen(addr); err != nil {
			return err
		}
	}
	return s.Serve(ctx)
}

// Serve accepts connections until ctx is canceled, then waits for open
// sessions to finish.
func (s *Server) Serve(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		s.Close()
	}()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				s.wg.Wait()
				return nil
			}
			return err
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			newSession(s, conn).serve()
		}()
	}
}

// Close stops accepting new connections. Sessions already open run to
// completion.
func (s *Server) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}
