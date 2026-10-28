package server

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/podu2997/cckvm/internal/layout"
	"github.com/podu2997/cckvm/internal/platform"
	"github.com/podu2997/cckvm/internal/platform/platformtest"
	"github.com/podu2997/cckvm/internal/protocol"
)

const twoScreens = `
section: screens
	desktop:
	laptop:
end
section: links
	desktop:
		right = laptop
	laptop:
		left = desktop
end
`

var desktopShape = platform.Rect{W: 1920, H: 1080}

type harness struct {
	t       *testing.T
	srv     *Server
	primary *platformtest.Fake
	addr    string
	stop    context.CancelFunc
	done    chan error
}

func start(t *testing.T, cfgText string, mod func(*Config)) *harness {
	t.Helper()
	lay, err := layout.Parse(strings.NewReader(cfgText))
	if err != nil {
		t.Fatal(err)
	}
	h := &harness{t: t, primary: platformtest.NewFake(desktopShape), done: make(chan error, 1)}
	cfg := Config{Layout: lay, Name: "desktop", Screen: h.primary, KeepAlive: time.Second}
	if mod != nil {
		mod(&cfg)
	}
	h.srv, err = New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	h.addr = ln.Addr().String()
	ctx, cancel := context.WithCancel(context.Background())
	h.stop = cancel
	go func() { h.done <- h.srv.Serve(ctx, ln) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-h.done:
		case <-time.After(3 * time.Second):
			t.Error("server did not stop")
		}
	})
	return h
}

// rawClient is a hand-driven Barrier client, used to check the server's
// bytes on the wire exactly as a genuine Barrier client would see them.
type rawClient struct {
	t    *testing.T
	conn *protocol.Conn
}

func dialRaw(t *testing.T, addr string) *rawClient {
	t.Helper()
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return &rawClient{t: t, conn: protocol.NewConn(c)}
}

func (r *rawClient) send(m protocol.Message) {
	r.t.Helper()
	if err := r.conn.Send(m); err != nil {
		r.t.Fatal(err)
	}
}

// next returns the next message other than keep-alives.
func (r *rawClient) next() protocol.Message {
	r.t.Helper()
	for {
		m, err := r.conn.Recv(3 * time.Second)
		if err != nil {
			r.t.Fatalf("recv: %v", err)
		}
		if _, ok := m.(protocol.KeepAlive); ok {
			continue
		}
		return m
	}
}

func expect[T protocol.Message](r *rawClient) T {
	r.t.Helper()
	m := r.next()
	v, ok := m.(T)
	if !ok {
		var zero T
		r.t.Fatalf("got %#v, want %T", m, zero)
	}
	return v
}

// join does the full handshake as screen name with the given size.
func (h *harness) join(name string, w, hgt int16) *rawClient {
	h.t.Helper()
	r := dialRaw(h.t, h.addr)
	expect[protocol.Hello](r)
	r.send(protocol.HelloBack{Major: 1, Minor: 6, Name: name})
	expect[protocol.QueryInfo](r)
	r.send(protocol.Info{W: w, H: hgt, MX: w / 2, MY: hgt / 2})
	expect[protocol.InfoAck](r)
	expect[protocol.ResetOptions](r)
	return r
}

func TestHandshakeBytes(t *testing.T) {
	h := start(t, twoScreens, nil)
	c, err := net.Dial("tcp", h.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(3 * time.Second))
	hello := make([]byte, 15)
	if _, err := io.ReadFull(c, hello); err != nil {
		t.Fatal(err)
	}
	if string(hello) != "\x00\x00\x00\x0bBarrier\x00\x01\x00\x06" {
		t.Fatalf("hello = %q", hello)
	}
	// A HelloBack exactly as Barrier 2.4 writes it.
	c.Write([]byte("\x00\x00\x00\x15Barrier\x00\x01\x00\x06\x00\x00\x00\x06laptop"))
	qinf := make([]byte, 8)
	if _, err := io.ReadFull(c, qinf); err != nil {
		t.Fatal(err)
	}
	if string(qinf) != "\x00\x00\x00\x04QINF" {
		t.Fatalf("qinf = %q", qinf)
	}
}

func TestIncompatibleVersion(t *testing.T) {
	h := start(t, twoScreens, nil)
	for _, v := range [][2]uint16{{1, 5}, {2, 0}} {
		r := dialRaw(t, h.addr)
		expect[protocol.Hello](r)
		r.send(protocol.HelloBack{Major: v[0], Minor: v[1], Name: "laptop"})
		if m := expect[protocol.Incompatible](r); m != (protocol.Incompatible{Major: 1, Minor: 6}) {
			t.Fatalf("EICV = %#v", m)
		}
		if _, err := r.conn.Recv(time.Second); !errors.Is(err, io.EOF) {
			t.Fatalf("connection not closed after EICV: %v", err)
		}
	}
	// A newer minor is accepted.
	r := dialRaw(t, h.addr)
	expect[protocol.Hello](r)
	r.send(protocol.HelloBack{Major: 1, Minor: 8, Name: "laptop"})
	expect[protocol.QueryInfo](r)
}

func TestUnknownAndBusy(t *testing.T) {
	h := start(t, twoScreens, nil)
	r := dialRaw(t, h.addr)
	expect[protocol.Hello](r)
	r.send(protocol.HelloBack{Major: 1, Minor: 6, Name: "ghost"})
	expect[protocol.Unknown](r)

	r = dialRaw(t, h.addr)
	expect[protocol.Hello](r)
	r.send(protocol.HelloBack{Major: 1, Minor: 6, Name: "desktop"})
	expect[protocol.Busy](r)

	h.join("laptop", 1280, 800)
	r = dialRaw(t, h.addr)
	expect[protocol.Hello](r)
	r.send(protocol.HelloBack{Major: 1, Minor: 6, Name: "LAPTOP"})
	expect[protocol.QueryInfo](r)
	r.send(protocol.Info{W: 10, H: 10})
	expect[protocol.Busy](r)
}

func TestBadGreeting(t *testing.T) {
	h := start(t, twoScreens, nil)
	r := dialRaw(t, h.addr)
	expect[protocol.Hello](r)
	r.send(protocol.KeepAlive{})
	expect[protocol.Bad](r)
}

func TestSwitchAndForwardInput(t *testing.T) {
	h := start(t, twoScreens, nil)
	lap := h.join("laptop", 1280, 800)

	// Push the local cursor against the right edge.
	h.primary.Emit(platform.Event{Kind: platform.MouseMotion, X: 1919, Y: 540})
	enter := expect[protocol.Enter](lap)
	if enter.X != 1 || enter.Y != 540*799/1079 || enter.Seq != 1 {
		t.Fatalf("enter = %#v", enter)
	}
	h.primary.Expect(t, "capture true")

	h.primary.Emit(platform.Event{Kind: platform.MouseDelta, DX: 10, DY: -5})
	if m := expect[protocol.MouseMove](lap); m.X != 11 || m.Y != enter.Y-5 {
		t.Fatalf("move = %#v", m)
	}
	h.primary.Emit(platform.Event{Kind: platform.Key, Down: true, KeyID: protocol.KeyShiftL, Mask: protocol.ModShift, KeyButton: 42})
	h.primary.Emit(platform.Event{Kind: platform.Key, Down: true, KeyID: 'A', Mask: protocol.ModShift, KeyButton: 30})
	h.primary.Emit(platform.Event{Kind: platform.Key, Down: true, Repeat: true, KeyID: 'A', Mask: protocol.ModShift, KeyButton: 30})
	h.primary.Emit(platform.Event{Kind: platform.Key, KeyID: 'A', Mask: protocol.ModShift, KeyButton: 30})
	if m := expect[protocol.KeyDown](lap); m != (protocol.KeyDown{ID: protocol.KeyShiftL, Mask: protocol.ModShift, Button: 42}) {
		t.Fatalf("%#v", m)
	}
	if m := expect[protocol.KeyDown](lap); m.ID != 'A' || m.Button != 30 {
		t.Fatalf("%#v", m)
	}
	if m := expect[protocol.KeyRepeat](lap); m.Count != 1 || m.ID != 'A' {
		t.Fatalf("%#v", m)
	}
	if m := expect[protocol.KeyUp](lap); m.ID != 'A' {
		t.Fatalf("%#v", m)
	}
	h.primary.Emit(platform.Event{Kind: platform.MouseButton, Button: protocol.ButtonLeft, Down: true})
	h.primary.Emit(platform.Event{Kind: platform.MouseButton, Button: protocol.ButtonLeft})
	h.primary.Emit(platform.Event{Kind: platform.MouseWheel, DY: -protocol.WheelDelta})
	expect[protocol.MouseDown](lap)
	expect[protocol.MouseUp](lap)
	if m := expect[protocol.MouseWheel](lap); m.DY != -120 {
		t.Fatalf("%#v", m)
	}

	// Clamped at the bottom: no message for a move that goes nowhere.
	h.primary.Emit(platform.Event{Kind: platform.MouseDelta, DY: 5000})
	if m := expect[protocol.MouseMove](lap); m.Y != 799 {
		t.Fatalf("clamp: %#v", m)
	}

	// Back out through the left edge.
	h.primary.Emit(platform.Event{Kind: platform.MouseDelta, DX: -20})
	expect[protocol.Leave](lap)
	h.primary.Expect(t, "capture false")
	if got := h.primary.Expect(t, "move"); got != "move 1918 1078" {
		t.Fatalf("return position: %s", got)
	}
	// Keys typed locally are no longer forwarded.
	h.primary.Emit(platform.Event{Kind: platform.Key, Down: true, KeyID: 'x'})
	h.primary.Emit(platform.Event{Kind: platform.MouseMotion, X: 1919, Y: 0})
	if m := expect[protocol.Enter](lap); m.Seq != 2 || m.Y != 1 {
		t.Fatalf("second enter = %#v (a key leaked?)", m)
	}
}

func TestRelativeMoves(t *testing.T) {
	h := start(t, twoScreens, func(c *Config) { c.Relative = true })
	lap := h.join("laptop", 1280, 800)
	h.primary.Emit(platform.Event{Kind: platform.MouseMotion, X: 1919, Y: 0})
	expect[protocol.Enter](lap)
	h.primary.Emit(platform.Event{Kind: platform.MouseDelta, DX: 7, DY: 3})
	if m := expect[protocol.MouseRelMove](lap); m != (protocol.MouseRelMove{DX: 7, DY: 3}) {
		t.Fatalf("%#v", m)
	}
	// Deltas are clipped at the edge so the server's idea of the position
	// matches the client's.
	h.primary.Emit(platform.Event{Kind: platform.MouseDelta, DY: -100})
	if m := expect[protocol.MouseRelMove](lap); m != (protocol.MouseRelMove{DX: 0, DY: -4}) {
		t.Fatalf("%#v", m)
	}
}

func TestNoSwitchWhileButtonHeldOrLocked(t *testing.T) {
	h := start(t, twoScreens, nil)
	lap := h.join("laptop", 1280, 800)
	h.primary.Emit(platform.Event{Kind: platform.MouseButton, Button: 1, Down: true})
	h.primary.Emit(platform.Event{Kind: platform.MouseMotion, X: 1919, Y: 5})
	h.primary.Emit(platform.Event{Kind: platform.MouseButton, Button: 1})
	h.primary.Emit(platform.Event{Kind: platform.Key, Down: true, KeyID: protocol.KeyScrollLock, Mask: protocol.ModScrollLock})
	h.primary.Emit(platform.Event{Kind: platform.MouseMotion, X: 1919, Y: 6})
	h.primary.Emit(platform.Event{Kind: platform.Key, KeyID: protocol.KeyScrollLock, Mask: 0})
	h.primary.Emit(platform.Event{Kind: platform.MouseMotion, X: 1919, Y: 7})
	if m := expect[protocol.Enter](lap); m.Y != 7*799/1079 {
		t.Fatalf("switched too early: %#v", m)
	}
}

func TestUnlinkedOrDisconnectedNeighbour(t *testing.T) {
	h := start(t, twoScreens, nil)
	// laptop is linked but not connected; left has no link at all.
	h.primary.Emit(platform.Event{Kind: platform.MouseMotion, X: 1919, Y: 5})
	h.primary.Emit(platform.Event{Kind: platform.MouseMotion, X: 0, Y: 5})
	h.primary.ExpectNone(t, "capture", 200*time.Millisecond)
}

func TestActiveClientDisconnect(t *testing.T) {
	h := start(t, twoScreens, nil)
	lap := h.join("laptop", 1280, 800)
	h.primary.Emit(platform.Event{Kind: platform.MouseMotion, X: 1919, Y: 5})
	expect[protocol.Enter](lap)
	h.primary.Expect(t, "capture true")
	lap.conn.Close()
	h.primary.Expect(t, "capture false")
	if got := h.primary.Expect(t, "move"); got != "move 960 540" {
		t.Fatalf("%s", got)
	}
	// The name is free again.
	h.join("laptop", 1280, 800)
}

func TestKeepAlive(t *testing.T) {
	h := start(t, twoScreens, func(c *Config) { c.KeepAlive = 50 * time.Millisecond })
	silent := h.join("laptop", 100, 100)
	// The server sends CALV every interval...
	got := 0
	t0 := time.Now()
	for got < 2 {
		m, err := silent.conn.Recv(time.Second)
		if err != nil {
			break
		}
		if _, ok := m.(protocol.KeepAlive); ok {
			got++
		}
	}
	if got < 2 {
		t.Fatal("no keep-alives")
	}
	// ...and drops a client that never answers within 3 intervals.
	for {
		_, err := silent.conn.Recv(time.Second)
		if err != nil {
			if time.Since(t0) > 2*time.Second {
				t.Fatalf("dropped too late: %v", time.Since(t0))
			}
			break
		}
	}

	// A client that answers stays connected.
	time.Sleep(100 * time.Millisecond) // let the server notice the drop
	chatty := h.join("laptop", 100, 100)
	deadline := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		m, err := chatty.conn.Recv(time.Second)
		if err != nil {
			t.Fatalf("responsive client dropped: %v", err)
		}
		if _, ok := m.(protocol.KeepAlive); ok {
			chatty.send(protocol.KeepAlive{})
		}
		chatty.send(protocol.Noop{})
	}
}

func TestShutdownSendsClose(t *testing.T) {
	h := start(t, twoScreens, nil)
	lap := h.join("laptop", 100, 100)
	h.primary.Emit(platform.Event{Kind: platform.MouseMotion, X: 1919, Y: 5})
	expect[protocol.Enter](lap)
	h.stop()
	expect[protocol.Close](lap)
	h.primary.Expect(t, "capture false")
}

func TestScreenSaver(t *testing.T) {
	h := start(t, twoScreens, nil)
	lap := h.join("laptop", 100, 100)
	h.primary.Emit(platform.Event{Kind: platform.MouseMotion, X: 1919, Y: 5})
	expect[protocol.Enter](lap)
	h.primary.Emit(platform.Event{Kind: platform.ScreenSaver, On: true})
	if m := expect[protocol.ScreenSaver](lap); !m.On {
		t.Fatal("want on")
	}
	// Barrier jumps back to the primary when its screensaver starts.
	expect[protocol.Leave](lap)
	h.primary.Emit(platform.Event{Kind: platform.ScreenSaver, On: false})
	if m := expect[protocol.ScreenSaver](lap); m.On {
		t.Fatal("want off")
	}
}

func TestInfoUpdate(t *testing.T) {
	h := start(t, twoScreens, nil)
	lap := h.join("laptop", 1280, 800)
	lap.send(protocol.Info{W: 640, H: 480})
	expect[protocol.InfoAck](lap)
	h.primary.Emit(platform.Event{Kind: platform.MouseMotion, X: 1919, Y: 1079})
	if m := expect[protocol.Enter](lap); m.Y != 478 {
		t.Fatalf("new shape not used: %#v", m)
	}
}

func TestUnknownMessagesIgnored(t *testing.T) {
	h := start(t, twoScreens, nil)
	lap := h.join("laptop", 1280, 800)
	lap.send(rawMessage("ZZZZsomething"))
	lap.send(protocol.Info{W: 10, H: 10})
	expect[protocol.InfoAck](lap) // still alive and in sync
}

// rawMessage sends arbitrary bytes as one frame.
type rawMessage []byte

func (m rawMessage) Code() string    { return string(m[:4]) }
func (m rawMessage) Marshal() []byte { return m }

func TestClipboardFromPrimary(t *testing.T) {
	h := start(t, twoScreens, nil)
	lap := h.join("laptop", 1280, 800)
	h.primary.CopyText(protocol.ClipboardMain, "hello from desktop")
	if m := expect[protocol.Grab](lap); m.ID != 0 {
		t.Fatalf("%#v", m)
	}
	h.primary.Emit(platform.Event{Kind: platform.MouseMotion, X: 1919, Y: 5})
	expect[protocol.Enter](lap)
	var asm protocol.Assembler
	for {
		m := expect[protocol.ClipboardData](lap)
		data, done, err := asm.Add(m)
		if err != nil {
			t.Fatal(err)
		}
		if done {
			f, _ := protocol.UnmarshalClipboard(data)
			if f[protocol.FormatText] != "hello from desktop" {
				t.Fatalf("got %q", f)
			}
			break
		}
	}
	// Re-entering does not resend unchanged data.
	h.primary.Emit(platform.Event{Kind: platform.MouseDelta, DX: -100})
	expect[protocol.Leave](lap)
	h.primary.Emit(platform.Event{Kind: platform.MouseMotion, X: 1919, Y: 5})
	expect[protocol.Enter](lap)
	h.primary.Emit(platform.Event{Kind: platform.MouseDelta, DX: 1})
	expect[protocol.MouseMove](lap)
}

func TestClipboardFromClient(t *testing.T) {
	h := start(t, twoScreens, nil)
	lap := h.join("laptop", 1280, 800)
	h.primary.Emit(platform.Event{Kind: platform.MouseMotion, X: 1919, Y: 5})
	enter := expect[protocol.Enter](lap)
	lap.send(protocol.Grab{ID: 0, Seq: enter.Seq})
	for _, m := range protocol.ChunkClipboard(0, enter.Seq, protocol.MarshalClipboard(map[uint32]string{
		protocol.FormatText: "from laptop", protocol.FormatHTML: "<i>from laptop</i>",
	})) {
		lap.send(m)
	}
	if got := h.primary.Expect(t, "setclip"); got != "setclip 0 from laptop" {
		t.Fatal(got)
	}
	// A stale grab (older sequence number) is ignored.
	lap.send(protocol.Grab{ID: 0, Seq: 0})
	for _, m := range protocol.ChunkClipboard(0, 0, protocol.TextClipboard("stale")) {
		lap.send(m)
	}
	h.primary.ExpectNone(t, "setclip", 200*time.Millisecond)
}

const lShaped = `
section: screens
	desktop:
	laptop:
	tablet:
end
section: links
	desktop:
		right = laptop
		down = tablet
	laptop:
		left = desktop
		down = tablet
	tablet:
		up = desktop
end
`

func TestThreeScreensAndClipboardRelay(t *testing.T) {
	h := start(t, lShaped, nil)
	lap := h.join("laptop", 1000, 1000)
	tab := h.join("tablet", 800, 600)

	h.primary.Emit(platform.Event{Kind: platform.MouseMotion, X: 1919, Y: 500})
	enter := expect[protocol.Enter](lap)
	// The laptop copies something.
	lap.send(protocol.Grab{ID: 0, Seq: enter.Seq})
	if m := expect[protocol.Grab](tab); m.Seq != enter.Seq {
		t.Fatalf("tablet not told of grab: %#v", m)
	}
	// Laptop to tablet directly (remote to remote), carrying the data the
	// laptop sends on COUT.
	h.primary.Emit(platform.Event{Kind: platform.MouseDelta, DX: 400, DY: 10000})
	expect[protocol.Leave](lap)
	for _, m := range protocol.ChunkClipboard(0, enter.Seq, protocol.TextClipboard("shared")) {
		lap.send(m)
	}
	if m := expect[protocol.Enter](tab); m.Y != 1 || m.X != (1+400)*799/999 {
		t.Fatalf("tablet enter = %#v", m)
	}
	if h.primary.Captured() != true {
		t.Fatal("capture released on a remote-to-remote switch")
	}
	// The tablet receives the laptop's clipboard while active.
	for _, want := range []uint8{protocol.MarkStart, protocol.MarkChunk, protocol.MarkEnd} {
		if m := expect[protocol.ClipboardData](tab); m.Mark != want {
			t.Fatalf("%#v", m)
		}
	}
	if got := h.primary.Expect(t, "setclip"); got != "setclip 0 shared" {
		t.Fatal(got)
	}
	// Up from the tablet lands on the desktop.
	h.primary.Emit(platform.Event{Kind: platform.MouseDelta, DY: -5000})
	expect[protocol.Leave](tab)
	h.primary.Expect(t, "capture false")
}

func TestEntryPoint(t *testing.T) {
	a := platform.Rect{W: 1920, H: 1080}
	b := platform.Rect{X: -100, Y: 50, W: 1280, H: 800}
	cases := []struct {
		d      layout.Direction
		x, y   int
		wx, wy int
	}{
		{layout.Right, 1919, 0, -99, 51}, // kept off the top edge
		{layout.Right, 1919, 1079, -99, 848},
		{layout.Left, 0, 540, 1178, 50 + 540*799/1079},
		{layout.Down, 1919, 1079, 1178, 51},
		{layout.Up, 0, 0, -99, 848},
	}
	for _, c := range cases {
		x, y := entryPoint(a, b, c.d, c.x, c.y)
		if x != c.wx || y != c.wy {
			t.Errorf("%s from (%d,%d): got (%d,%d) want (%d,%d)", c.d, c.x, c.y, x, y, c.wx, c.wy)
		}
	}
}

func TestNewValidates(t *testing.T) {
	lay, _ := layout.Parse(strings.NewReader(twoScreens))
	if _, err := New(Config{Layout: lay, Name: "nobody", Screen: platformtest.NewFake(desktopShape)}); err == nil {
		t.Fatal("unknown server name accepted")
	}
	if _, err := New(Config{Layout: lay, Name: "desktop"}); err == nil {
		t.Fatal("missing screen accepted")
	}
}
