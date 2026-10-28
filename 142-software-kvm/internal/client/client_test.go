package client_test

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/podu2997/cckvm/internal/client"
	"github.com/podu2997/cckvm/internal/layout"
	"github.com/podu2997/cckvm/internal/platform"
	"github.com/podu2997/cckvm/internal/platform/platformtest"
	"github.com/podu2997/cckvm/internal/protocol"
	"github.com/podu2997/cckvm/internal/server"
)

var laptopShape = platform.Rect{W: 1280, H: 800}

// rawServer is a hand-driven Barrier server.
type rawServer struct {
	t  *testing.T
	ln net.Listener
}

func listen(t *testing.T) *rawServer {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	return &rawServer{t, ln}
}

func (s *rawServer) accept() *peer {
	s.t.Helper()
	c, err := s.ln.Accept()
	if err != nil {
		s.t.Fatal(err)
	}
	s.t.Cleanup(func() { c.Close() })
	return &peer{s.t, protocol.NewConn(c)}
}

type peer struct {
	t    *testing.T
	conn *protocol.Conn
}

func (p *peer) send(ms ...protocol.Message) {
	p.t.Helper()
	for _, m := range ms {
		if err := p.conn.Send(m); err != nil {
			p.t.Fatal(err)
		}
	}
}

func recv[T protocol.Message](p *peer) T {
	p.t.Helper()
	m, err := p.conn.Recv(3 * time.Second)
	if err != nil {
		p.t.Fatal(err)
	}
	v, ok := m.(T)
	if !ok {
		var zero T
		p.t.Fatalf("got %#v, want %T", m, zero)
	}
	return v
}

type running struct {
	cancel context.CancelFunc
	done   chan error
}

func run(t *testing.T, cfg client.Config) *running {
	ctx, cancel := context.WithCancel(context.Background())
	r := &running{cancel, make(chan error, 1)}
	go func() { r.done <- client.Run(ctx, cfg) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-r.done:
		case <-time.After(3 * time.Second):
			t.Error("client did not stop")
		}
	})
	return r
}

func (r *running) wait(t *testing.T) error {
	t.Helper()
	select {
	case err := <-r.done:
		r.done <- err // keep it for Cleanup
		return err
	case <-time.After(3 * time.Second):
		t.Fatal("client still running")
		return nil
	}
}

func handshake(p *peer) {
	p.t.Helper()
	p.send(protocol.Hello{Major: 1, Minor: 6})
	if hb := recv[protocol.HelloBack](p); hb != (protocol.HelloBack{Major: 1, Minor: 6, Name: "laptop"}) {
		p.t.Fatalf("HelloBack = %#v", hb)
	}
	p.send(protocol.QueryInfo{})
	info := recv[protocol.Info](p)
	if info.W != 1280 || info.H != 800 || info.MX != 640 || info.MY != 400 {
		p.t.Fatalf("DINF = %#v", info)
	}
	p.send(protocol.InfoAck{}, protocol.ResetOptions{}, protocol.SetOptions{Options: []uint32{1, 2}})
}

func TestClientReplaysServerMessages(t *testing.T) {
	srv := listen(t)
	screen := platformtest.NewFake(laptopShape)
	r := run(t, client.Config{Name: "laptop", Addr: srv.ln.Addr().String(), Screen: screen})
	p := srv.accept()
	handshake(p)

	p.send(protocol.Enter{X: 1, Y: 300, Seq: 4, Mask: protocol.ModShift})
	screen.Expect(t, "enter 1 300 1")
	p.send(
		protocol.MouseMove{X: 20, Y: -1},
		protocol.MouseRelMove{DX: -3, DY: 4},
		protocol.KeyDown{ID: 'a', Mask: 0, Button: 30},
		protocol.KeyRepeat{ID: 'a', Mask: 0, Count: 2, Button: 30},
		protocol.KeyUp{ID: 'a', Mask: 0, Button: 30},
		protocol.MouseDown{Button: 3},
		protocol.MouseUp{Button: 3},
		protocol.MouseWheel{DX: 120, DY: -240},
		protocol.ScreenSaver{On: true},
	)
	for _, want := range []string{
		"move 20 -1", "relmove -3 4", "keydown 97 0 30", "keyrepeat 97 0 2 30", "keyup 97 0 30",
		"button 3 true", "button 3 false", "wheel 120 -240", "screensaver true",
	} {
		if got := screen.Expect(t, strings.Fields(want)[0]); got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	}

	p.send(protocol.KeepAlive{})
	recv[protocol.KeepAlive](p)

	for _, m := range protocol.ChunkClipboard(0, 4, protocol.TextClipboard("pasted")) {
		p.send(m)
	}
	screen.Expect(t, "setclip 0 pasted")
	p.send(protocol.Leave{})
	screen.Expect(t, "leave")

	p.send(protocol.Close{})
	if err := r.wait(t); err != nil {
		t.Fatalf("CBYE should end cleanly, got %v", err)
	}
}

func TestClientClipboardGrab(t *testing.T) {
	srv := listen(t)
	screen := platformtest.NewFake(laptopShape)
	run(t, client.Config{Name: "laptop", Addr: srv.ln.Addr().String(), Screen: screen})
	p := srv.accept()
	handshake(p)

	p.send(protocol.Enter{X: 1, Y: 1, Seq: 9})
	screen.Expect(t, "enter")
	screen.CopyText(0, "copied on laptop")
	if g := recv[protocol.Grab](p); g != (protocol.Grab{ID: 0, Seq: 9}) {
		t.Fatalf("grab = %#v", g)
	}
	// Data follows on leave.
	p.send(protocol.Leave{})
	if got := readClipboard(p); got != "copied on laptop" {
		t.Fatalf("got %q", got)
	}

	// While inactive, a change is sent right away.
	screen.CopyText(0, "background copy")
	recv[protocol.Grab](p)
	if got := readClipboard(p); got != "background copy" {
		t.Fatalf("got %q", got)
	}

	// Text that came from the server is not echoed back.
	for _, m := range protocol.ChunkClipboard(0, 9, protocol.TextClipboard("from server")) {
		p.send(m)
	}
	screen.Expect(t, "setclip")
	screen.Emit(platform.Event{Kind: platform.ClipboardChanged, ClipboardID: 0})
	p.send(protocol.QueryInfo{})
	recv[protocol.Info](p) // the next message is the DINF, not a CCLP
}

func readClipboard(p *peer) string {
	p.t.Helper()
	var a protocol.Assembler
	for {
		data, done, err := a.Add(recv[protocol.ClipboardData](p))
		if err != nil {
			p.t.Fatal(err)
		}
		if done {
			f, err := protocol.UnmarshalClipboard(data)
			if err != nil {
				p.t.Fatal(err)
			}
			return f[protocol.FormatText]
		}
	}
}

func TestClientErrors(t *testing.T) {
	cases := []struct {
		name   string
		script func(p *peer)
		want   error
	}{
		{"old server", func(p *peer) { p.send(protocol.Hello{Major: 1, Minor: 5}) }, client.ErrIncompatible},
		{"EUNK", func(p *peer) {
			p.send(protocol.Hello{Major: 1, Minor: 6})
			recv[protocol.HelloBack](p)
			p.send(protocol.Unknown{})
		}, client.ErrUnknownName},
		{"EICV", func(p *peer) {
			p.send(protocol.Hello{Major: 1, Minor: 6})
			recv[protocol.HelloBack](p)
			p.send(protocol.Incompatible{Major: 1, Minor: 6})
		}, client.ErrIncompatible},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := listen(t)
			// Reconnect is on: these errors must still be final.
			r := run(t, client.Config{Name: "laptop", Addr: srv.ln.Addr().String(),
				Screen: platformtest.NewFake(laptopShape), Reconnect: true, RetryDelay: 10 * time.Millisecond})
			tc.script(srv.accept())
			if err := r.wait(t); !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
		})
	}
}

func TestClientKeepAliveTimeout(t *testing.T) {
	srv := listen(t)
	r := run(t, client.Config{Name: "laptop", Addr: srv.ln.Addr().String(),
		Screen: platformtest.NewFake(laptopShape), KeepAlive: 50 * time.Millisecond})
	handshake(srv.accept())
	start := time.Now()
	err := r.wait(t)
	if err == nil || !strings.Contains(err.Error(), "keep-alive") {
		t.Fatalf("got %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("took %v", time.Since(start))
	}
}

func TestClientReconnects(t *testing.T) {
	srv := listen(t)
	run(t, client.Config{Name: "laptop", Addr: srv.ln.Addr().String(),
		Screen: platformtest.NewFake(laptopShape), Reconnect: true, RetryDelay: 10 * time.Millisecond})
	first := srv.accept()
	first.send(protocol.Hello{Major: 1, Minor: 6})
	recv[protocol.HelloBack](first)
	first.conn.Close() // server crashes
	second := srv.accept()
	handshake(second)
	second.send(protocol.Busy{}) // EBSY is retried too
	handshake(srv.accept())
}

// TestEndToEnd runs the real server and the real client against each other.
func TestEndToEnd(t *testing.T) {
	lay, err := layout.Parse(strings.NewReader(`
section: screens
	desktop:
	laptop:
end
section: links
	desktop:
		left = laptop
	laptop:
		right = desktop
end`))
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	desk := platformtest.NewFake(platform.Rect{W: 1920, H: 1080})
	srv, err := server.New(server.Config{Layout: lay, Name: "desktop", Screen: desk, Logger: logger})
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, stopServer := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- srv.Serve(ctx, ln) }()
	defer stopServer()

	lap := platformtest.NewFake(laptopShape)
	r := run(t, client.Config{Name: "laptop", Addr: ln.Addr().String(), Screen: lap, Logger: logger})

	desk.CopyText(0, "desktop text")
	// Wait for the client to register: retry the edge push until it enters.
	deadline := time.Now().Add(3 * time.Second)
	for !desk.Captured() {
		if time.Now().After(deadline) {
			t.Fatal("never switched to the laptop")
		}
		desk.Emit(platform.Event{Kind: platform.MouseMotion, X: 0, Y: 540})
		time.Sleep(20 * time.Millisecond)
	}
	if got := lap.Expect(t, "enter"); got != "enter 1278 399 0" {
		t.Fatalf("got %s", got)
	}
	lap.Expect(t, "setclip 0 desktop text")

	desk.Emit(platform.Event{Kind: platform.MouseDelta, DX: -100, DY: 10})
	lap.Expect(t, "move 1178 409")
	desk.Emit(platform.Event{Kind: platform.Key, Down: true, KeyID: 'q', KeyButton: 16})
	desk.Emit(platform.Event{Kind: platform.Key, KeyID: 'q', KeyButton: 16})
	lap.Expect(t, "keydown 113 0 16")
	lap.Expect(t, "keyup 113 0 16")

	lap.CopyText(0, "laptop text")
	time.Sleep(50 * time.Millisecond) // let the grab reach the server first
	desk.Emit(platform.Event{Kind: platform.MouseDelta, DX: 500})
	lap.Expect(t, "leave")
	desk.Expect(t, "capture false")
	desk.Expect(t, "setclip 0 laptop text")

	stopServer()
	if err := r.wait(t); err != nil {
		t.Fatalf("client after CBYE: %v", err)
	}
	if err := <-served; err != nil {
		t.Fatal(err)
	}
}
