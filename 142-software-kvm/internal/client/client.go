// Package client is the Barrier-compatible KVM client: it connects to a
// server, describes its screen, and replays the server's keyboard, mouse,
// clipboard and screensaver messages on the local desktop.
package client

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/podu2997/cckvm/internal/platform"
	"github.com/podu2997/cckvm/internal/protocol"
)

// Config configures a client.
type Config struct {
	// Name is this screen's name in the server's layout.
	Name string
	// Addr is host:port of the server (port 24800 by default).
	Addr   string
	Screen platform.Screen
	// Dial opens the connection. Defaults to a plain TCP dial of Addr; set it
	// to add TLS.
	Dial func(ctx context.Context, addr string) (net.Conn, error)
	// KeepAlive is the server's CALV interval; three intervals of silence
	// mean the server is gone. Defaults to 3 seconds.
	KeepAlive time.Duration
	// Reconnect keeps retrying after recoverable failures.
	Reconnect bool
	// RetryDelay is the first wait between attempts; it doubles up to 30s.
	RetryDelay time.Duration
	Logger     *slog.Logger
}

// Errors the server can end a session with.
var (
	ErrIncompatible = errors.New("server rejected our protocol version (EICV)")
	ErrUnknownName  = errors.New("server does not know this screen name (EUNK)")
	ErrBusy         = errors.New("server already has a screen with this name (EBSY)")
	ErrBadMessage   = errors.New("server says we sent a bad message (EBAD)")
	ErrServerClosed = errors.New("server closed the session (CBYE)")
)

// fatal errors are not retried: the configuration has to change first.
func fatal(err error) bool {
	return errors.Is(err, ErrIncompatible) || errors.Is(err, ErrUnknownName) || errors.Is(err, ErrBadMessage)
}

// Run connects and serves sessions until ctx is cancelled or, without
// Reconnect, the first session ends.
func Run(ctx context.Context, cfg Config) error {
	if cfg.Screen == nil || cfg.Name == "" || cfg.Addr == "" {
		return errors.New("client: Name, Addr and Screen are required")
	}
	if cfg.KeepAlive <= 0 {
		cfg.KeepAlive = 3 * time.Second
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}
	if cfg.Dial == nil {
		cfg.Dial = func(ctx context.Context, addr string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "tcp", addr)
		}
	}
	delay := cfg.RetryDelay
	for {
		start := time.Now()
		err := runSession(ctx, cfg)
		if ctx.Err() != nil {
			return nil
		}
		if !cfg.Reconnect || fatal(err) {
			if errors.Is(err, ErrServerClosed) {
				return nil
			}
			return err
		}
		if time.Since(start) > 30*time.Second {
			delay = cfg.RetryDelay // the session was healthy; start over
		}
		cfg.Logger.Warn("session ended, reconnecting", "err", err, "in", delay)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(delay):
		}
		delay = min(delay*2, 30*time.Second)
	}
}

type session struct {
	cfg    Config
	conn   *protocol.Conn
	log    *slog.Logger
	active bool
	acked  bool // the server has accepted our DINF at least once
	seq    uint32
	asm    protocol.Assembler
	// owned[id]: we grabbed clipboard id and the server has not taken it back.
	owned [protocol.NumClipboards]bool
	// sent[id]: the server already has our data for clipboard id.
	sent [protocol.NumClipboards]bool
	// known[id]: the text last put on or read from clipboard id, used to
	// ignore change notifications caused by our own SetClipboard.
	known [protocol.NumClipboards]string
}

type received struct {
	msg protocol.Message
	err error
}

func runSession(ctx context.Context, cfg Config) error {
	c, err := cfg.Dial(ctx, cfg.Addr)
	if err != nil {
		return err
	}
	s := &session{cfg: cfg, conn: protocol.NewConn(c), log: cfg.Logger.With("server", cfg.Addr)}
	defer s.conn.Close()

	if err := s.hello(); err != nil {
		return err
	}
	s.log.Debug("greeted server", "screen", cfg.Name)

	timeout := 3 * cfg.KeepAlive
	msgs := make(chan received, 64)
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			m, err := s.conn.Recv(timeout)
			var unknown *protocol.UnknownMessageError
			if errors.As(err, &unknown) {
				s.log.Debug("ignoring message", "code", unknown.Code)
				continue
			}
			select {
			case msgs <- received{m, err}:
			case <-done:
				return
			}
			if err != nil {
				return
			}
		}
	}()

	local := cfg.Screen.Events()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case r := <-msgs:
			if r.err != nil {
				var ne net.Error
				if errors.As(r.err, &ne) && ne.Timeout() {
					return fmt.Errorf("no keep-alive from server for %v", timeout)
				}
				return r.err
			}
			if err := s.handle(r.msg); err != nil {
				return err
			}
		case e := <-local:
			if err := s.onLocal(e); err != nil {
				return err
			}
		}
	}
}

func (s *session) hello() error {
	m, err := s.conn.Recv(10 * time.Second)
	if err != nil {
		return fmt.Errorf("waiting for Hello: %w", err)
	}
	h, ok := m.(protocol.Hello)
	if !ok {
		return fmt.Errorf("expected Hello, got %s", m.Code())
	}
	if h.Major != protocol.MajorVersion || h.Minor < protocol.MinorVersion {
		return fmt.Errorf("%w: server speaks %d.%d, need %d.%d+", ErrIncompatible,
			h.Major, h.Minor, protocol.MajorVersion, protocol.MinorVersion)
	}
	return s.conn.Send(protocol.HelloBack{Major: protocol.MajorVersion, Minor: protocol.MinorVersion, Name: s.cfg.Name})
}

func (s *session) info() protocol.Info {
	r := s.cfg.Screen.Shape()
	x, y := s.cfg.Screen.CursorPos()
	return protocol.Info{X: int16(r.X), Y: int16(r.Y), W: int16(r.W), H: int16(r.H), MX: int16(x), MY: int16(y)}
}

func (s *session) handle(m protocol.Message) error {
	sc := s.cfg.Screen
	switch m := m.(type) {
	case protocol.QueryInfo:
		return s.conn.Send(s.info())
	case protocol.InfoAck:
		if !s.acked {
			s.acked = true
			s.log.Info("connected", "screen", s.cfg.Name)
		}
	case protocol.Noop, protocol.ResetOptions:
	case protocol.SetOptions:
		s.log.Debug("server options", "options", m.Options)
	case protocol.KeepAlive:
		return s.conn.Send(protocol.KeepAlive{})
	case protocol.Enter:
		s.active, s.seq = true, m.Seq
		sc.Enter(int(m.X), int(m.Y), m.Mask)
	case protocol.Leave:
		for id := range s.owned {
			if s.owned[id] && !s.sent[id] {
				if err := s.sendClipboard(uint8(id)); err != nil {
					return err
				}
			}
		}
		s.active = false
		sc.Leave()
	case protocol.Grab:
		if int(m.ID) < len(s.owned) {
			s.owned[m.ID] = false
		}
	case protocol.ClipboardData:
		data, done, err := s.asm.Add(m)
		if err != nil {
			s.log.Warn("bad clipboard transfer", "err", err)
			return nil
		}
		if !done {
			return nil
		}
		formats, err := protocol.UnmarshalClipboard(data)
		if err != nil {
			s.log.Warn("bad clipboard data", "err", err)
			return nil
		}
		if text, ok := formats[protocol.FormatText]; ok {
			s.known[m.ID] = text
			s.owned[m.ID] = false
			sc.SetClipboard(m.ID, text)
		}
	case protocol.KeyDown:
		sc.KeyDown(m.ID, m.Mask, m.Button)
	case protocol.KeyUp:
		sc.KeyUp(m.ID, m.Mask, m.Button)
	case protocol.KeyRepeat:
		sc.KeyRepeat(m.ID, m.Mask, m.Count, m.Button)
	case protocol.MouseDown:
		sc.MouseButton(m.Button, true)
	case protocol.MouseUp:
		sc.MouseButton(m.Button, false)
	case protocol.MouseMove:
		sc.MouseMove(int(m.X), int(m.Y))
	case protocol.MouseRelMove:
		sc.MouseRelMove(int(m.DX), int(m.DY))
	case protocol.MouseWheel:
		sc.MouseWheel(int(m.DX), int(m.DY))
	case protocol.ScreenSaver:
		sc.ScreenSaver(m.On)
	case protocol.Close:
		return ErrServerClosed
	case protocol.Incompatible:
		return fmt.Errorf("%w: server speaks %d.%d", ErrIncompatible, m.Major, m.Minor)
	case protocol.Busy:
		return ErrBusy
	case protocol.Unknown:
		return ErrUnknownName
	case protocol.Bad:
		return ErrBadMessage
	default:
		s.log.Debug("ignoring message", "code", m.Code())
	}
	return nil
}

func (s *session) onLocal(e platform.Event) error {
	switch e.Kind {
	case platform.ShapeChanged:
		return s.conn.Send(s.info())
	case platform.ClipboardChanged:
		id := e.ClipboardID
		if int(id) >= protocol.NumClipboards {
			return nil
		}
		text, ok := s.cfg.Screen.Clipboard(id)
		if !ok || text == s.known[id] {
			return nil
		}
		s.known[id] = text
		s.owned[id], s.sent[id] = true, false
		if err := s.conn.Send(protocol.Grab{ID: id, Seq: s.seq}); err != nil {
			return err
		}
		// While active the data goes with COUT; otherwise send it now so
		// the server has it before the next switch.
		if !s.active {
			return s.sendClipboard(id)
		}
	}
	return nil
}

func (s *session) sendClipboard(id uint8) error {
	s.sent[id] = true
	for _, m := range protocol.ChunkClipboard(id, s.seq, protocol.TextClipboard(s.known[id])) {
		if err := s.conn.Send(m); err != nil {
			return err
		}
	}
	return nil
}
