// Package server is the Barrier-compatible KVM server. It owns the real
// keyboard and mouse, decides which screen is active, and forwards input
// to remote screens over the Barrier 1.6 protocol.
//
// All state lives in one goroutine (loop). Connection goroutines do the
// handshake and then turn each received message into an event for it.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/podu2997/cckvm/internal/layout"
	"github.com/podu2997/cckvm/internal/platform"
	"github.com/podu2997/cckvm/internal/protocol"
)

// Config configures a Server.
type Config struct {
	Layout *layout.Config
	// Name is this machine's screen in Layout.
	Name string
	// Screen is the local desktop (the primary screen).
	Screen platform.Screen
	// KeepAlive is the CALV interval; a client silent for three intervals
	// is dropped. Defaults to Barrier's 3 seconds.
	KeepAlive time.Duration
	// Relative sends DMRM deltas instead of DMMV positions while a remote
	// screen is active.
	Relative bool
	// HandshakeTimeout bounds Hello..DINF. Defaults to 10 seconds.
	HandshakeTimeout time.Duration
	Logger           *slog.Logger
}

// Server is a running KVM server.
type Server struct {
	cfg    Config
	log    *slog.Logger
	events chan event

	// Owned by loop.
	primary *screen
	screens map[string]*screen
	active  *screen
	x, y    int    // cursor position on the active screen
	seq     uint32 // incremented on every enter into a remote screen
	mask    uint16 // modifier state of the local keyboard
	buttons uint32 // bit n set while mouse button n is held
	clip    [protocol.NumClipboards]clipboard
}

type screen struct {
	name  string
	shape platform.Rect
	proxy *proxy // nil for the primary screen
	// dirty[id] is set when the screen has not been sent the current
	// contents of clipboard id.
	dirty [protocol.NumClipboards]bool
}

type proxy struct {
	conn     *protocol.Conn
	name     string
	lastRecv time.Time
	asm      protocol.Assembler
}

type clipboard struct {
	owner string // screen that last grabbed it
	seq   uint32 // the owner's enter sequence number at the time of the grab
	data  string // marshalled Barrier clipboard; "" until known
}

type event struct {
	kind  int
	p     *proxy
	info  protocol.Info
	msg   protocol.Message
	reply chan error
}

const (
	evRegister = iota
	evMessage
	evGone
)

// ErrBusy is returned to a client whose name is already connected.
var ErrBusy = errors.New("screen name already in use")

// New validates cfg and creates a server.
func New(cfg Config) (*Server, error) {
	if cfg.Layout == nil || cfg.Screen == nil {
		return nil, errors.New("server: Layout and Screen are required")
	}
	name, ok := cfg.Layout.Canonical(cfg.Name)
	if !ok {
		return nil, fmt.Errorf("server: screen %q is not in the layout (have %v)", cfg.Name, cfg.Layout.Screens())
	}
	cfg.Name = name
	if cfg.KeepAlive <= 0 {
		cfg.KeepAlive = 3 * time.Second
	}
	if cfg.HandshakeTimeout <= 0 {
		cfg.HandshakeTimeout = 10 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}
	s := &Server{cfg: cfg, log: cfg.Logger, events: make(chan event, 64), screens: map[string]*screen{}}
	s.primary = &screen{name: name, shape: cfg.Screen.Shape()}
	s.screens[name] = s.primary
	s.active = s.primary
	s.x, s.y = cfg.Screen.CursorPos()
	return s, nil
}

// Serve accepts clients on ln until ctx is cancelled, then says CBYE to
// every client and returns.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() { <-ctx.Done(); ln.Close() }()

	loopDone := make(chan struct{})
	go func() { s.loop(ctx); close(loopDone) }()

	var acceptErr error
	for {
		c, err := ln.Accept()
		if err != nil {
			if ctx.Err() == nil {
				acceptErr = err
				cancel()
			}
			break
		}
		go s.handle(ctx, c)
	}
	<-loopDone
	return acceptErr
}

// send delivers an event to the loop unless the server is shutting down.
func (s *Server) send(ctx context.Context, e event) bool {
	select {
	case s.events <- e:
		return true
	case <-ctx.Done():
		return false
	}
}

// handle runs one connection: handshake, registration, then a read loop.
func (s *Server) handle(ctx context.Context, c net.Conn) {
	conn := protocol.NewConn(c)
	defer conn.Close()
	log := s.log.With("remote", c.RemoteAddr().String())

	name, info, err := s.handshake(conn)
	if err != nil {
		log.Warn("handshake failed", "err", err)
		return
	}
	log = log.With("screen", name)
	p := &proxy{conn: conn, name: name, lastRecv: time.Now()}
	reply := make(chan error, 1)
	if !s.send(ctx, event{kind: evRegister, p: p, info: info, reply: reply}) {
		return
	}
	select {
	case err := <-reply:
		if err != nil {
			log.Warn("rejected", "err", err)
			conn.Send(protocol.Busy{})
			return
		}
	case <-ctx.Done():
		return
	}
	for {
		m, err := conn.Recv(0)
		var unknown *protocol.UnknownMessageError
		if errors.As(err, &unknown) {
			log.Debug("ignoring message", "code", unknown.Code)
			continue
		}
		if err != nil {
			s.send(ctx, event{kind: evGone, p: p, msg: nil})
			log.Info("disconnected", "err", err)
			return
		}
		if !s.send(ctx, event{kind: evMessage, p: p, msg: m}) {
			return
		}
	}
}

// handshake runs Hello / HelloBack / QINF / DINF and returns the client's
// canonical screen name and screen info. Protocol errors are reported to
// the client with EICV, EUNK, EBSY or EBAD before returning.
func (s *Server) handshake(conn *protocol.Conn) (string, protocol.Info, error) {
	to := s.cfg.HandshakeTimeout
	if err := conn.Send(protocol.Hello{Major: protocol.MajorVersion, Minor: protocol.MinorVersion}); err != nil {
		return "", protocol.Info{}, err
	}
	m, err := conn.Recv(to)
	if err != nil {
		var unknown *protocol.UnknownMessageError
		if errors.As(err, &unknown) {
			conn.Send(protocol.Bad{})
		}
		return "", protocol.Info{}, err
	}
	hb, ok := m.(protocol.HelloBack)
	if !ok {
		conn.Send(protocol.Bad{})
		return "", protocol.Info{}, fmt.Errorf("expected HelloBack, got %s", m.Code())
	}
	// Newer minors are backwards compatible; older ones need adapters
	// (ClientProxy1_0..1_5 in Barrier) that this server does not have.
	if hb.Major != protocol.MajorVersion || hb.Minor < protocol.MinorVersion {
		conn.Send(protocol.Incompatible{Major: protocol.MajorVersion, Minor: protocol.MinorVersion})
		return "", protocol.Info{}, fmt.Errorf("client %q speaks protocol %d.%d, need %d.%d+",
			hb.Name, hb.Major, hb.Minor, protocol.MajorVersion, protocol.MinorVersion)
	}
	name, ok := s.cfg.Layout.Canonical(hb.Name)
	if !ok {
		conn.Send(protocol.Unknown{})
		return "", protocol.Info{}, fmt.Errorf("unknown screen %q", hb.Name)
	}
	if name == s.cfg.Name {
		conn.Send(protocol.Busy{})
		return "", protocol.Info{}, fmt.Errorf("client claims the server's own name %q", name)
	}
	if err := conn.Send(protocol.QueryInfo{}); err != nil {
		return "", protocol.Info{}, err
	}
	deadline := time.Now().Add(to)
	for {
		m, err := conn.Recv(time.Until(deadline))
		var unknown *protocol.UnknownMessageError
		if errors.As(err, &unknown) {
			continue
		}
		if err != nil {
			return "", protocol.Info{}, fmt.Errorf("waiting for DINF: %w", err)
		}
		if info, ok := m.(protocol.Info); ok {
			return name, info, nil
		}
	}
}

func (s *Server) loop(ctx context.Context) {
	tick := time.NewTicker(s.cfg.KeepAlive)
	defer tick.Stop()
	local := s.cfg.Screen.Events()
	for {
		select {
		case <-ctx.Done():
			s.shutdown()
			return
		case e := <-s.events:
			s.onEvent(e)
		case e, ok := <-local:
			if !ok {
				local = nil
				continue
			}
			s.onLocal(e)
		case <-tick.C:
			s.keepAlive()
		}
	}
}

func (s *Server) shutdown() {
	if s.active != s.primary {
		s.cfg.Screen.Capture(false)
	}
	for _, sc := range s.screens {
		if sc.proxy != nil {
			sc.proxy.conn.Send(protocol.Close{})
			sc.proxy.conn.Close()
		}
	}
	// Unblock any handler still waiting for a registration reply.
	for {
		select {
		case e := <-s.events:
			if e.reply != nil {
				e.reply <- errors.New("server shutting down")
			}
		default:
			return
		}
	}
}

func (s *Server) onEvent(e event) {
	switch e.kind {
	case evRegister:
		if _, taken := s.screens[e.p.name]; taken {
			e.reply <- ErrBusy
			return
		}
		sc := &screen{name: e.p.name, proxy: e.p, shape: infoRect(e.info)}
		for id := range s.clip {
			sc.dirty[id] = s.clip[id].data != ""
		}
		s.screens[sc.name] = sc
		e.reply <- nil
		s.sendTo(sc, protocol.InfoAck{})
		s.sendTo(sc, protocol.ResetOptions{})
		s.log.Info("client connected", "screen", sc.name, "shape", sc.shape.String())
	case evMessage:
		sc := s.screens[e.p.name]
		if sc == nil || sc.proxy != e.p {
			return
		}
		e.p.lastRecv = time.Now()
		s.onClientMessage(sc, e.msg)
	case evGone:
		sc := s.screens[e.p.name]
		if sc == nil || sc.proxy != e.p {
			return
		}
		delete(s.screens, sc.name)
		if s.active == sc {
			x, y := s.primary.shape.Center()
			s.log.Info("active screen disconnected, returning to primary", "screen", sc.name)
			s.switchTo(s.primary, x, y)
		}
	}
}

func (s *Server) onClientMessage(sc *screen, m protocol.Message) {
	switch m := m.(type) {
	case protocol.KeepAlive, protocol.Noop:
	case protocol.Info:
		sc.shape = infoRect(m)
		s.sendTo(sc, protocol.InfoAck{})
		if s.active == sc {
			s.x, s.y = sc.shape.Clamp(s.x, s.y)
		}
	case protocol.Grab:
		s.grab(sc, m.ID, m.Seq)
	case protocol.ClipboardData:
		data, done, err := sc.proxy.asm.Add(m)
		if err != nil {
			s.log.Warn("bad clipboard transfer", "screen", sc.name, "err", err)
			return
		}
		if done {
			s.clipboardData(sc, m.ID, m.Seq, data)
		}
	default:
		s.log.Debug("ignoring message from client", "screen", sc.name, "code", m.Code())
	}
}

func (s *Server) keepAlive() {
	now := time.Now()
	for _, sc := range s.screens {
		if sc.proxy == nil {
			continue
		}
		if now.Sub(sc.proxy.lastRecv) > 3*s.cfg.KeepAlive {
			s.log.Warn("client timed out", "screen", sc.name)
			// The read loop fails and reports evGone.
			sc.proxy.conn.Close()
			continue
		}
		s.sendTo(sc, protocol.KeepAlive{})
	}
}

func (s *Server) sendTo(sc *screen, m protocol.Message) {
	if sc.proxy == nil {
		return
	}
	if err := sc.proxy.conn.Send(m); err != nil {
		s.log.Warn("send failed", "screen", sc.name, "code", m.Code(), "err", err)
		sc.proxy.conn.Close()
	}
}

func (s *Server) broadcast(m protocol.Message, except string) {
	for _, sc := range s.screens {
		if sc.name != except {
			s.sendTo(sc, m)
		}
	}
}

func infoRect(i protocol.Info) platform.Rect {
	return platform.Rect{X: int(i.X), Y: int(i.Y), W: int(i.W), H: int(i.H)}
}
