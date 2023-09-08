package server

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// startTestServer boots a server on an ephemeral loopback port and returns its
// address plus a cleanup function.
func startTestServer(t *testing.T) (string, func()) {
	t.Helper()
	srv := New(Options{
		Addr:   "127.0.0.1",
		Port:   0,
		Logger: log.New(io.Discard, "", 0),
	})
	if err := srv.Listen(); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	go func() { _ = srv.Serve() }()
	addr := srv.Addr().String()
	return addr, func() { srv.Shutdown() }
}

// conn wraps a client connection with a buffered reader for line/byte reads.
type conn struct {
	t *testing.T
	c net.Conn
	r *bufio.Reader
}

func dial(t *testing.T, addr string) *conn {
	t.Helper()
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return &conn{t: t, c: c, r: bufio.NewReader(c)}
}

func (c *conn) send(s string) {
	c.t.Helper()
	if _, err := c.c.Write([]byte(s)); err != nil {
		c.t.Fatalf("write: %v", err)
	}
}

// readLine reads one CRLF-terminated line (with a deadline) and returns it
// without the trailing CRLF.
func (c *conn) readLine() string {
	c.t.Helper()
	_ = c.c.SetReadDeadline(time.Now().Add(2 * time.Second))
	line, err := c.r.ReadString('\n')
	if err != nil {
		c.t.Fatalf("readLine: %v (got %q)", err, line)
	}
	return strings.TrimRight(line, "\r\n")
}

// readN reads exactly n bytes (used for MSG payloads).
func (c *conn) readN(n int) []byte {
	c.t.Helper()
	_ = c.c.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, n)
	if _, err := io.ReadFull(c.r, buf); err != nil {
		c.t.Fatalf("readN(%d): %v", n, err)
	}
	return buf
}

// expectLine asserts the next line equals want.
func (c *conn) expectLine(want string) {
	c.t.Helper()
	if got := c.readLine(); got != want {
		c.t.Fatalf("line = %q, want %q", got, want)
	}
}

// expectInfo consumes and validates the greeting INFO line.
func (c *conn) expectInfo() {
	c.t.Helper()
	line := c.readLine()
	if !strings.HasPrefix(line, "INFO {") {
		c.t.Fatalf("first line = %q, want INFO {...}", line)
	}
}

// expectNoData asserts nothing arrives within a short window.
func (c *conn) expectNoData() {
	c.t.Helper()
	_ = c.c.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if _, err := c.r.ReadByte(); err == nil {
		c.t.Fatalf("expected no data, but got some")
	}
}

func (c *conn) close() { _ = c.c.Close() }

func TestConnectFlow(t *testing.T) {
	addr, stop := startTestServer(t)
	defer stop()

	c := dial(t, addr)
	defer c.close()

	c.expectInfo()
	c.send("CONNECT {}\r\n")
	c.expectLine("+OK")
	c.send("PING\r\n")
	c.expectLine("PONG")
}

func TestVerboseFalseSuppressesOK(t *testing.T) {
	addr, stop := startTestServer(t)
	defer stop()

	c := dial(t, addr)
	defer c.close()
	c.expectInfo()
	c.send("CONNECT {\"verbose\":false}\r\n")
	// No +OK should arrive; PING still answered.
	c.send("PING\r\n")
	c.expectLine("PONG")
}

func TestPubSubDelivery(t *testing.T) {
	addr, stop := startTestServer(t)
	defer stop()

	sub := dial(t, addr)
	defer sub.close()
	sub.expectInfo()
	sub.send("CONNECT {\"verbose\":false}\r\n")
	sub.send("SUB foo 1\r\n")

	pub := dial(t, addr)
	defer pub.close()
	pub.expectInfo()
	pub.send("CONNECT {\"verbose\":false}\r\n")
	pub.send("PUB foo 5\r\nhello\r\n")

	sub.expectLine("MSG foo 1 5")
	if got := string(sub.readN(5)); got != "hello" {
		t.Fatalf("payload = %q, want hello", got)
	}
	sub.expectLine("") // trailing CRLF after payload
}

func TestMultipleSubscribersAndIsolation(t *testing.T) {
	addr, stop := startTestServer(t)
	defer stop()

	mk := func(subject, sid string) *conn {
		c := dial(t, addr)
		c.expectInfo()
		c.send("CONNECT {\"verbose\":false}\r\n")
		c.send(fmt.Sprintf("SUB %s %s\r\n", subject, sid))
		return c
	}

	a := mk("foo", "1")
	defer a.close()
	b := mk("foo", "2")
	defer b.close()
	other := mk("bar", "9")
	defer other.close()

	pub := dial(t, addr)
	defer pub.close()
	pub.expectInfo()
	pub.send("CONNECT {\"verbose\":false}\r\n")
	pub.send("PUB foo 2\r\nhi\r\n")

	for _, c := range []*conn{a, b} {
		c.expectLine("MSG foo " + sidOf(c, a, b) + " 2")
		if got := string(c.readN(2)); got != "hi" {
			t.Fatalf("payload = %q", got)
		}
		c.expectLine("")
	}
	other.expectNoData()
}

// sidOf returns "1" for a and "2" for b (subscriptions created in that order).
func sidOf(c, a, b *conn) string {
	if c == a {
		return "1"
	}
	return "2"
}

func TestUnsubStopsDeliveryButConnStaysOpen(t *testing.T) {
	addr, stop := startTestServer(t)
	defer stop()

	sub := dial(t, addr)
	defer sub.close()
	sub.expectInfo()
	sub.send("CONNECT {\"verbose\":false}\r\n")
	sub.send("SUB foo 1\r\n")

	pub := dial(t, addr)
	defer pub.close()
	pub.expectInfo()
	pub.send("CONNECT {\"verbose\":false}\r\n")

	// First publish is delivered.
	pub.send("PUB foo 1\r\nA\r\n")
	sub.expectLine("MSG foo 1 1")
	sub.readN(1)
	sub.expectLine("")

	// Unsubscribe, then publish again: no MSG.
	sub.send("UNSUB 1\r\n")
	// small settle so the UNSUB is processed before the next PUB races it
	time.Sleep(50 * time.Millisecond)
	pub.send("PUB foo 1\r\nB\r\n")

	// Connection still answers PING.
	sub.send("PING\r\n")
	sub.expectLine("PONG")
}

func TestUnsubWithMaxMsgs(t *testing.T) {
	addr, stop := startTestServer(t)
	defer stop()

	sub := dial(t, addr)
	defer sub.close()
	sub.expectInfo()
	sub.send("CONNECT {\"verbose\":false}\r\n")
	sub.send("SUB foo 1\r\n")
	sub.send("UNSUB 1 2\r\n") // auto-remove after 2 more messages

	pub := dial(t, addr)
	defer pub.close()
	pub.expectInfo()
	pub.send("CONNECT {\"verbose\":false}\r\n")

	time.Sleep(50 * time.Millisecond)
	for i := 0; i < 5; i++ {
		pub.send("PUB foo 1\r\nX\r\n")
	}

	// Exactly two messages should be delivered.
	for i := 0; i < 2; i++ {
		sub.expectLine("MSG foo 1 1")
		sub.readN(1)
		sub.expectLine("")
	}
	sub.send("PING\r\n")
	sub.expectLine("PONG") // proves no third MSG is queued ahead of PONG
}

func TestBinarySafePayload(t *testing.T) {
	addr, stop := startTestServer(t)
	defer stop()

	sub := dial(t, addr)
	defer sub.close()
	sub.expectInfo()
	sub.send("CONNECT {\"verbose\":false}\r\n")
	sub.send("SUB foo 1\r\n")

	pub := dial(t, addr)
	defer pub.close()
	pub.expectInfo()
	pub.send("CONNECT {\"verbose\":false}\r\n")

	payload := "line1\r\nline2 with spaces\r\n"
	pub.send(fmt.Sprintf("PUB foo %d\r\n%s\r\n", len(payload), payload))

	sub.expectLine(fmt.Sprintf("MSG foo 1 %d", len(payload)))
	if got := string(sub.readN(len(payload))); got != payload {
		t.Fatalf("payload mismatch: got %q want %q", got, payload)
	}
	sub.expectLine("")
}

func TestUnknownProtocolOp(t *testing.T) {
	addr, stop := startTestServer(t)
	defer stop()

	c := dial(t, addr)
	defer c.close()
	c.expectInfo()
	c.send("BOGUS stuff\r\n")
	c.expectLine("-ERR 'Unknown Protocol Operation'")
}

// TestConcurrentFanout: many subscribers, one publisher fans out M messages;
// each subscriber must receive exactly M. Run under -race.
func TestConcurrentFanout(t *testing.T) {
	addr, stop := startTestServer(t)
	defer stop()

	const subscribers = 10
	const messages = 50

	var wg sync.WaitGroup
	ready := make(chan struct{})
	errCh := make(chan error, subscribers)

	subs := make([]*conn, subscribers)
	for i := 0; i < subscribers; i++ {
		c := dial(t, addr)
		c.expectInfo()
		c.send("CONNECT {\"verbose\":false}\r\n")
		c.send("SUB foo 1\r\n")
		subs[i] = c
	}
	// Give the server a moment to register all subscriptions.
	time.Sleep(100 * time.Millisecond)

	for i := 0; i < subscribers; i++ {
		wg.Add(1)
		go func(c *conn) {
			defer wg.Done()
			<-ready
			for m := 0; m < messages; m++ {
				line := c.readLine()
				if !strings.HasPrefix(line, "MSG foo 1 ") {
					errCh <- fmt.Errorf("unexpected line %q", line)
					return
				}
				var n int
				if _, err := fmt.Sscanf(line, "MSG foo 1 %d", &n); err != nil {
					errCh <- err
					return
				}
				c.readN(n)
				c.expectLine("")
			}
		}(subs[i])
	}

	pub := dial(t, addr)
	defer pub.close()
	pub.expectInfo()
	pub.send("CONNECT {\"verbose\":false}\r\n")

	close(ready)
	for m := 0; m < messages; m++ {
		pub.send(fmt.Sprintf("PUB foo 3\r\nm%02d\r\n", m%100))
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("subscriber error: %v", err)
	}
	for _, c := range subs {
		c.close()
	}
}
