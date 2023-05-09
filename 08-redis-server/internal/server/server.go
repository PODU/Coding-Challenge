// Package server serves the store over TCP, one goroutine per connection.
package server

import (
	"bufio"
	"io"
	"log"
	"net"
	"time"

	"rediserver/internal/resp"
	"rediserver/internal/store"
)

// Server serves one store to any number of clients.
type Server struct {
	Store  *store.Store
	DBFile string // snapshot path used by SAVE

	Logger *log.Logger
}

// New returns a Server for st. SAVE writes to dbfile.
func New(st *store.Store, dbfile string, logger *log.Logger) *Server {
	if logger == nil {
		logger = log.Default()
	}
	return &Server{Store: st, DBFile: dbfile, Logger: logger}
}

// ListenAndServe binds addr and serves until the listener is closed.
func (s *Server) ListenAndServe(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.Logger.Printf("listening on %s", ln.Addr())
	return s.Serve(ln)
}

// Serve handles connections from ln until Accept fails, and returns that error.
func (s *Server) Serve(ln net.Listener) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go s.handleConn(conn)
	}
}

// handleConn serves one client until it disconnects or sends malformed input.
// Panics are recovered so a bad request only kills its own connection.
func (s *Server) handleConn(conn net.Conn) {
	defer func() {
		if r := recover(); r != nil {
			s.Logger.Printf("recovered panic on %s: %v", conn.RemoteAddr(), r)
		}
		conn.Close()
	}()

	r := resp.NewReader(bufio.NewReader(conn))
	w := resp.NewWriter(bufio.NewWriter(conn))

	for {
		v, err := r.Read()
		if err != nil {
			if err == io.EOF {
				return
			}
			if resp.IsProtocolError(err) || err == io.ErrUnexpectedEOF {
				_ = w.Write(resp.ErrorString("ERR Protocol error: " + err.Error()))
				_ = w.Flush()
			}
			return
		}

		reply, ok := s.dispatch(v)
		if !ok {
			continue
		}
		if err := w.Write(reply); err != nil {
			return
		}
		if err := w.Flush(); err != nil {
			return
		}
	}
}

// RunActiveExpiry purges expired keys every interval until stop is closed.
// Reads already expire keys lazily; this just frees memory for keys nobody
// reads again.
func (s *Server) RunActiveExpiry(interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			s.Store.PurgeExpired()
		}
	}
}
