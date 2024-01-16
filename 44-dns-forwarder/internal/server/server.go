// Package server wires the UDP listener, message parsing, cache, blocklist, and
// upstream forwarder into a running DNS forwarder.
package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"dns-forwarder/internal/blocklist"
	"dns-forwarder/internal/cache"
	"dns-forwarder/internal/dnsmsg"
)

const (
	readBufSize  = 65535
	blockTTL     = 300 // seconds, for sinkhole answers
	defaultLimit = 1024
)

// Exchanger forwards a raw query to an upstream resolver and returns the raw
// response. *upstream.Client satisfies it; tests inject a fake.
type Exchanger interface {
	Exchange(ctx context.Context, query []byte) ([]byte, error)
}

// Config holds server settings. The cache and blocklist are optional (nil
// disables them).
type Config struct {
	Listen   string        // UDP listen address (host:port)
	Timeout  time.Duration // per-query upstream timeout
	Sinkhole string        // "nxdomain" or "zero"
	Logger   *slog.Logger
}

// Counters are cumulative, atomic request counters.
type Counters struct {
	Queries   atomic.Int64
	Hits      atomic.Int64
	Forwards  atomic.Int64
	Blocked   atomic.Int64
	Errors    atomic.Int64
	ServFails atomic.Int64
}

// Server is a running forwarder. Use New then Start/Shutdown.
type Server struct {
	cfg       Config
	upstream  Exchanger
	cache     *cache.Cache
	blocklist *blocklist.List
	log       *slog.Logger

	conn    *net.UDPConn
	closing atomic.Bool
	wg      sync.WaitGroup
	sem     chan struct{} // bounds concurrent in-flight handlers

	Counters Counters
}

// New constructs a Server. A nil cache disables caching; a nil blocklist
// disables blocking; a nil logger discards logs.
func New(cfg Config, up Exchanger, c *cache.Cache, bl *blocklist.List) *Server {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if cfg.Sinkhole == "" {
		cfg.Sinkhole = "nxdomain"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	return &Server{
		cfg:       cfg,
		upstream:  up,
		cache:     c,
		blocklist: bl,
		log:       logger,
		sem:       make(chan struct{}, defaultLimit),
	}
}

// Start binds the UDP socket and launches the read loop. It returns once the
// listener is ready (or fails to bind).
func (s *Server) Start() error {
	addr, err := net.ResolveUDPAddr("udp", s.cfg.Listen)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	s.conn = conn
	s.log.Info("listening", "addr", conn.LocalAddr().String())
	s.wg.Add(1)
	go s.readLoop()
	return nil
}

// LocalAddr returns the bound listen address (valid after Start).
func (s *Server) LocalAddr() net.Addr {
	if s.conn == nil {
		return nil
	}
	return s.conn.LocalAddr()
}

// Shutdown stops accepting traffic, closes the socket, and waits for in-flight
// handlers to finish or the context to expire.
func (s *Server) Shutdown(ctx context.Context) error {
	if !s.closing.CompareAndSwap(false, true) {
		return nil
	}
	if s.conn != nil {
		s.conn.Close()
	}
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Server) readLoop() {
	defer s.wg.Done()
	buf := make([]byte, readBufSize)
	for {
		n, clientAddr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			if s.closing.Load() {
				return
			}
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				continue
			}
			s.log.Error("read error", "err", err)
			return
		}
		pkt := make([]byte, n)
		copy(pkt, buf[:n])

		s.wg.Add(1)
		s.sem <- struct{}{}
		go func() {
			defer s.wg.Done()
			defer func() { <-s.sem }()
			s.handle(pkt, clientAddr)
		}()
	}
}

func (s *Server) handle(query []byte, client *net.UDPAddr) {
	defer func() {
		if r := recover(); r != nil {
			s.Counters.Errors.Add(1)
			s.log.Error("handler panic", "recover", r)
		}
	}()
	s.Counters.Queries.Add(1)

	msg, err := dnsmsg.Decode(query)
	if err != nil || len(msg.Questions) == 0 {
		s.Counters.Errors.Add(1)
		s.log.Warn("malformed query", "client", client.String(), "err", err)
		s.reply(dnsmsg.BuildResponse(query, dnsmsg.RCodeFormErr), client)
		return
	}

	q := msg.Questions[0]
	name := strings.ToLower(strings.TrimSuffix(q.Name, "."))
	s.log.Debug("query", "id", msg.Header.ID, "name", name,
		"type", q.Type, "class", q.Class, "rd", msg.Header.RD, "client", client.String())

	if s.blocklist.Blocked(name) {
		s.Counters.Blocked.Add(1)
		s.log.Info("blocked", "name", name, "mode", s.cfg.Sinkhole)
		s.reply(s.synthBlock(msg.Header, q), client)
		return
	}

	key := cache.NormalizeKey(name, q.Type, q.Class)

	if s.cache != nil {
		if cached, ok := s.cache.Get(key); ok {
			dnsmsg.SetID(cached, msg.Header.ID)
			s.Counters.Hits.Add(1)
			s.log.Debug("cache hit", "name", name)
			s.reply(cached, client)
			return
		}
	}

	s.Counters.Forwards.Add(1)
	s.log.Info("forwarding", "name", name, "type", q.Type, "upstream", true)

	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.Timeout)
	defer cancel()
	resp, err := s.upstream.Exchange(ctx, query)
	if err != nil {
		s.Counters.ServFails.Add(1)
		s.log.Warn("upstream failure", "name", name, "err", err)
		s.reply(dnsmsg.BuildResponse(query, dnsmsg.RCodeServFail), client)
		return
	}

	// Only successful answers are cached; errors and NXDOMAIN are not.
	if s.cache != nil {
		if rmsg, err := dnsmsg.Decode(resp); err == nil && rmsg.Header.RCode == dnsmsg.RCodeNoError {
			if ttl, ok := rmsg.MinAnswerTTL(); ok {
				s.cache.Put(key, resp, ttl)
			}
		}
	}
	s.reply(resp, client)
}

// synthBlock builds a local answer for a blocked name: NXDOMAIN, or a sinkhole
// A/AAAA record (0.0.0.0 / ::) when configured with -sinkhole zero.
func (s *Server) synthBlock(qh dnsmsg.Header, q dnsmsg.Question) []byte {
	if s.cfg.Sinkhole == "zero" && (q.Type == dnsmsg.TypeA || q.Type == dnsmsg.TypeAAAA) {
		var rdata []byte
		if q.Type == dnsmsg.TypeA {
			rdata = make([]byte, 4) // 0.0.0.0
		} else {
			rdata = make([]byte, 16) // ::
		}
		resp := &dnsmsg.Message{
			Header: dnsmsg.Header{
				ID: qh.ID, QR: true, RD: qh.RD, RA: true, RCode: dnsmsg.RCodeNoError,
			},
			Questions: []dnsmsg.Question{q},
			Answers: []dnsmsg.ResourceRecord{{
				Name: q.Name, Type: q.Type, Class: q.Class, TTL: blockTTL, RData: rdata,
			}},
		}
		if b, err := dnsmsg.Encode(resp); err == nil {
			return b
		}
	}

	resp := &dnsmsg.Message{
		Header: dnsmsg.Header{
			ID: qh.ID, QR: true, RD: qh.RD, RA: true, RCode: dnsmsg.RCodeNXDomain,
		},
		Questions: []dnsmsg.Question{q},
	}
	b, _ := dnsmsg.Encode(resp)
	return b
}

func (s *Server) reply(b []byte, client *net.UDPAddr) {
	if b == nil || s.conn == nil {
		return
	}
	if _, err := s.conn.WriteToUDP(b, client); err != nil {
		s.log.Warn("write failed", "client", client.String(), "err", err)
	}
}
