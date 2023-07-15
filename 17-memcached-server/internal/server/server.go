// Package server serves the memcached text protocol over TCP, one goroutine
// per connection.
package server

import (
	"bufio"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"sync/atomic"

	"memcached/internal/protocol"
	"memcached/internal/store"
)

// Config holds the server options.
type Config struct {
	Addr     string
	MaxConns int  // beyond this, accepting pauses until one closes
	Verbose  bool // log every command
}

// Server accepts connections and serves the memcached text protocol.
type Server struct {
	cfg   Config
	store *store.Store

	ln     net.Listener
	sem    chan struct{}
	closed atomic.Bool

	wg      sync.WaitGroup
	onceErr sync.Once
}

// New returns a Server; nothing is bound until ListenAndServe or Serve.
func New(cfg Config, st *store.Store) *Server {
	if cfg.MaxConns <= 0 {
		cfg.MaxConns = 1024
	}
	return &Server{
		cfg:   cfg,
		store: st,
		sem:   make(chan struct{}, cfg.MaxConns),
	}
}

// Addr returns the actual listening address (useful when binding to :0).
func (s *Server) Addr() net.Addr {
	if s.ln == nil {
		return nil
	}
	return s.ln.Addr()
}

// ListenAndServe binds the configured address and serves until Close.
func (s *Server) ListenAndServe() error {
	ln, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return err
	}
	return s.Serve(ln)
}

// Serve accepts connections on ln until Close is called.
func (s *Server) Serve(ln net.Listener) error {
	s.ln = ln
	if s.cfg.Verbose {
		log.Printf("listening on %s", ln.Addr())
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			if s.closed.Load() {
				break
			}
			return err
		}
		s.sem <- struct{}{}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer func() { <-s.sem }()
			s.handleConn(conn)
		}()
	}
	s.wg.Wait() // let in-flight connections drain
	return nil
}

// Close stops accepting new connections and unblocks Serve. In-flight
// connections are allowed to drain.
func (s *Server) Close() error {
	var err error
	s.onceErr.Do(func() {
		s.closed.Store(true)
		if s.ln != nil {
			err = s.ln.Close()
		}
	})
	return err
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReaderSize(conn, 16*1024)
	w := bufio.NewWriter(conn)

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}

		cmd, perr := protocol.Parse(line)
		if perr != nil {
			if perr.Client {
				io.WriteString(w, protocol.ClientError(perr.Msg))
			} else {
				io.WriteString(w, protocol.RespError)
			}
			if w.Flush() != nil {
				return
			}
			continue
		}

		var fatal bool
		switch cmd.Type {
		case protocol.TypeStorage:
			fatal = s.handleStorage(r, w, cmd)
		case protocol.TypeRetrieval:
			s.handleRetrieval(w, cmd)
		case protocol.TypeDelete:
			s.handleDelete(w, cmd)
		}
		if fatal {
			return
		}
		if w.Flush() != nil {
			return
		}
	}
}

// handleStorage reads the data block and applies the command. It returns true
// if the stream is broken and the connection should be dropped.
func (s *Server) handleStorage(r *bufio.Reader, w *bufio.Writer, cmd *protocol.Command) bool {
	buf := make([]byte, cmd.Bytes+2)
	if _, err := io.ReadFull(r, buf); err != nil {
		return true
	}

	if buf[cmd.Bytes] != '\r' || buf[cmd.Bytes+1] != '\n' {
		// The block wasn't the declared length. Skip to the next newline to try
		// to get back in sync.
		_, _ = r.ReadString('\n')
		if !cmd.NoReply {
			io.WriteString(w, protocol.ClientError("bad data chunk"))
		}
		return false
	}
	value := buf[:cmd.Bytes]

	var res store.Result
	switch cmd.Op {
	case "set":
		res = s.store.Set(cmd.Key, cmd.Flags, cmd.Exptime, value)
	case "add":
		res = s.store.Add(cmd.Key, cmd.Flags, cmd.Exptime, value)
	case "replace":
		res = s.store.Replace(cmd.Key, cmd.Flags, cmd.Exptime, value)
	case "append":
		res = s.store.Append(cmd.Key, value)
	case "prepend":
		res = s.store.Prepend(cmd.Key, value)
	case "cas":
		res = s.store.Cas(cmd.Key, cmd.Flags, cmd.Exptime, value, cmd.CASUnique)
	}

	if s.cfg.Verbose {
		log.Printf("%s %s (%d bytes) -> %s", cmd.Op, cmd.Key, cmd.Bytes, resultName(res))
	}
	if !cmd.NoReply {
		io.WriteString(w, storeResponse(res))
	}
	return false
}

func (s *Server) handleRetrieval(w *bufio.Writer, cmd *protocol.Command) {
	views := s.store.Get(cmd.Keys)
	for _, v := range views {
		// VALUE <key> <flags> <bytes> [<cas>]
		w.WriteString("VALUE ")
		w.WriteString(v.Key)
		w.WriteByte(' ')
		w.WriteString(strconv.FormatUint(uint64(v.Flags), 10))
		w.WriteByte(' ')
		w.WriteString(strconv.Itoa(len(v.Value)))
		if cmd.Op == "gets" {
			w.WriteByte(' ')
			w.WriteString(strconv.FormatUint(v.CAS, 10))
		}
		w.WriteString("\r\n")
		w.Write(v.Value)
		w.WriteString("\r\n")
	}
	w.WriteString(protocol.RespEnd)
	if s.cfg.Verbose {
		log.Printf("%s %v -> %d hit(s)", cmd.Op, cmd.Keys, len(views))
	}
}

func (s *Server) handleDelete(w *bufio.Writer, cmd *protocol.Command) {
	res := s.store.Delete(cmd.Key)
	if s.cfg.Verbose {
		log.Printf("delete %s -> %s", cmd.Key, resultName(res))
	}
	if cmd.NoReply {
		return
	}
	if res == store.ResultStored {
		w.WriteString(protocol.RespDeleted)
	} else {
		w.WriteString(protocol.RespNotFound)
	}
}

func storeResponse(res store.Result) string {
	switch res {
	case store.ResultStored:
		return protocol.RespStored
	case store.ResultNotStored:
		return protocol.RespNotStored
	case store.ResultExists:
		return protocol.RespExists
	case store.ResultNotFound:
		return protocol.RespNotFound
	case store.ResultTooLarge:
		return protocol.ServerError("object too large for cache")
	default:
		return protocol.ServerError("unknown error")
	}
}

func resultName(res store.Result) string {
	switch res {
	case store.ResultStored:
		return "STORED"
	case store.ResultNotStored:
		return "NOT_STORED"
	case store.ResultExists:
		return "EXISTS"
	case store.ResultNotFound:
		return "NOT_FOUND"
	case store.ResultTooLarge:
		return "TOO_LARGE"
	default:
		return "UNKNOWN"
	}
}
