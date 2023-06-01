package server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// connReadTimeout bounds how long a single connection may take to deliver its
// request line and headers, mitigating slow-loris style hangs.
const connReadTimeout = 15 * time.Second

// Server is an immutable static-file HTTP/1.1 server. Once constructed its
// fields never change, so handler goroutines need no synchronisation.
type Server struct {
	addr string
	root string // absolute, cleaned document root
}

// New constructs a Server, validating that dir exists and is a directory and
// resolving it to an absolute path used for traversal containment checks.
func New(addr, dir string) (*Server, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolving doc root %q: %w", dir, err)
	}
	abs = filepath.Clean(abs)

	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("doc root %q: %w", abs, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("doc root %q is not a directory", abs)
	}

	return &Server{addr: addr, root: abs}, nil
}

// Root returns the absolute document root (useful for tests).
func (s *Server) Root() string { return s.root }

// Run opens the listener and serves connections until ctx is cancelled, then
// stops accepting and waits for in-flight connections to finish.
func (s *Server) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.addr, err)
	}
	log.Printf("serving %s on http://%s", s.root, ln.Addr())

	return s.Serve(ctx, ln)
}

// Serve accepts connections on ln until ctx is cancelled. It is exported so
// tests can supply their own listener.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	var wg sync.WaitGroup

	// Close the listener when the context is cancelled so Accept unblocks.
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				wg.Wait()
				log.Print("shutdown complete")
				return nil
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return fmt.Errorf("accept: %w", err)
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			s.handle(conn)
		}()
	}
}

// handle serves a single connection: read deadline, parse, respond, close.
// It recovers from panics so one bad request can never crash the process.
func (s *Server) handle(conn net.Conn) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("recovered from panic serving %s: %v", conn.RemoteAddr(), r)
		}
		_ = conn.Close()
	}()

	_ = conn.SetReadDeadline(time.Now().Add(connReadTimeout))

	br := bufio.NewReader(conn)
	bw := bufio.NewWriter(conn)

	req, err := ParseRequest(br)
	if err != nil {
		// Only malformed input gets a 400; a dropped connection gets nothing.
		if errors.Is(err, errBadRequest) {
			_ = errorResponse(400, false).Write(bw)
		}
		return
	}

	resp := Handle(req, s.root)
	if err := resp.Write(bw); err != nil {
		log.Printf("writing response to %s: %v", conn.RemoteAddr(), err)
	}
}
