// Package server is the TCP side of the broker, one goroutine per connection.
package server

import (
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/codingchallenges/nats/internal/protocol"
	"github.com/codingchallenges/nats/internal/router"
)

// Options configures a Server.
type Options struct {
	Addr   string // host to bind, e.g. "0.0.0.0"
	Port   int
	Logger *log.Logger // defaults to log.Default()
}

// Server is a NATS broker.
type Server struct {
	opts     Options
	registry *router.Registry
	logger   *log.Logger

	mu       sync.Mutex
	ln       net.Listener
	clients  map[*Client]struct{}
	nextID   uint64
	closing  bool
	doneOnce sync.Once
	wg       sync.WaitGroup
}

// New returns a Server. Nothing is bound until Listen or Run.
func New(opts Options) *Server {
	if opts.Logger == nil {
		opts.Logger = log.Default()
	}
	return &Server{
		opts:     opts,
		registry: router.New(),
		logger:   opts.Logger,
		clients:  make(map[*Client]struct{}),
	}
}

// Addr returns the address actually bound (handy with port 0), or nil before
// Listen.
func (s *Server) Addr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln == nil {
		return nil
	}
	return s.ln.Addr()
}

// Listen binds the TCP socket; call Serve next.
func (s *Server) Listen() error {
	addr := fmt.Sprintf("%s:%d", s.opts.Addr, s.opts.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	s.mu.Lock()
	s.ln = ln
	s.mu.Unlock()
	s.logger.Printf("listening for client connections on %s", ln.Addr())
	return nil
}

// Serve accepts connections until Shutdown, which makes it return nil.
func (s *Server) Serve() error {
	s.mu.Lock()
	ln := s.ln
	s.mu.Unlock()
	if ln == nil {
		return fmt.Errorf("server: Listen must be called before Serve")
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			s.mu.Lock()
			closing := s.closing
			s.mu.Unlock()
			if closing {
				return nil
			}
			return fmt.Errorf("accept: %w", err)
		}
		s.handleConn(conn)
	}
}

// Run calls Listen and then Serve.
func (s *Server) Run() error {
	if err := s.Listen(); err != nil {
		return err
	}
	return s.Serve()
}

func (s *Server) handleConn(conn net.Conn) {
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		_ = conn.Close()
		return
	}
	s.nextID++
	id := s.nextID
	c := newClient(s, conn, id)
	s.clients[c] = struct{}{}
	s.wg.Add(1)
	s.mu.Unlock()

	s.logger.Printf("client %d connected from %s", id, conn.RemoteAddr())
	go func() {
		defer s.wg.Done()
		c.readLoop()
		s.removeClient(c)
		s.logger.Printf("client %d disconnected", id)
	}()
}

// removeClient drops c and all its subscriptions.
func (s *Server) removeClient(c *Client) {
	for _, sub := range c.takeAllSubs() {
		s.registry.Unsubscribe(sub)
	}
	s.mu.Lock()
	delete(s.clients, c)
	s.mu.Unlock()
}

// Shutdown stops accepting, closes every connection and waits for their
// goroutines to finish.
func (s *Server) Shutdown() {
	s.doneOnce.Do(func() {
		s.mu.Lock()
		s.closing = true
		if s.ln != nil {
			_ = s.ln.Close()
		}
		conns := make([]*Client, 0, len(s.clients))
		for c := range s.clients {
			conns = append(conns, c)
		}
		s.mu.Unlock()

		for _, c := range conns {
			c.close()
		}
		s.wg.Wait()
		s.logger.Printf("server shut down")
	})
}

func (s *Server) info(clientID uint64) protocol.Info {
	host, port := splitHostPort(s.Addr())
	return protocol.Info{
		ServerID:   "NATS-CC25",
		ServerName: "nats-cc25",
		Version:    "0.1.0",
		Proto:      1,
		Go:         "go1.24",
		Host:       host,
		Port:       port,
		MaxPayload: maxPayload,
		Headers:    false,
		ClientID:   clientID,
	}
}

func splitHostPort(addr net.Addr) (string, int) {
	if addr == nil {
		return "0.0.0.0", 0
	}
	if tcp, ok := addr.(*net.TCPAddr); ok {
		host := tcp.IP.String()
		if tcp.IP == nil || tcp.IP.IsUnspecified() {
			host = "0.0.0.0"
		}
		return host, tcp.Port
	}
	return "0.0.0.0", 0
}
