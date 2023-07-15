package server

import (
	"bufio"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"memcached/internal/store"
)

// startTestServer launches a server on an ephemeral port and returns its
// address plus a cleanup func.
func startTestServer(t *testing.T, st *store.Store) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := New(Config{MaxConns: 64}, st)
	done := make(chan struct{})
	go func() {
		_ = srv.Serve(ln)
		close(done)
	}()
	return ln.Addr().String(), func() {
		srv.Close()
		<-done
	}
}

// conn wraps a client connection with line helpers.
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

func (c *conn) line() string {
	c.t.Helper()
	c.c.SetReadDeadline(time.Now().Add(2 * time.Second))
	l, err := c.r.ReadString('\n')
	if err != nil {
		c.t.Fatalf("read: %v", err)
	}
	return strings.TrimRight(l, "\r\n")
}

func (c *conn) expect(want string) {
	c.t.Helper()
	if got := c.line(); got != want {
		c.t.Fatalf("expected %q, got %q", want, got)
	}
}

func (c *conn) close() { c.c.Close() }

func TestIntegrationBasicFlow(t *testing.T) {
	addr, stop := startTestServer(t, store.New(0))
	defer stop()
	c := dial(t, addr)
	defer c.close()

	c.send("set foo 0 0 3\r\nbar\r\n")
	c.expect("STORED")

	c.send("get foo\r\n")
	c.expect("VALUE foo 0 3")
	c.expect("bar")
	c.expect("END")

	c.send("gets foo\r\n")
	got := c.line()
	if !strings.HasPrefix(got, "VALUE foo 0 3 ") {
		t.Fatalf("gets missing cas token: %q", got)
	}
	c.expect("bar")
	c.expect("END")

	c.send("add foo 0 0 1\r\nx\r\n")
	c.expect("NOT_STORED")

	c.send("append foo 0 0 3\r\nbaz\r\n")
	c.expect("STORED")
	c.send("get foo\r\n")
	c.expect("VALUE foo 0 6")
	c.expect("barbaz")
	c.expect("END")

	c.send("delete foo\r\n")
	c.expect("DELETED")
	c.send("get foo\r\n")
	c.expect("END")
	c.send("delete foo\r\n")
	c.expect("NOT_FOUND")
}

func TestIntegrationMultiKeyGet(t *testing.T) {
	addr, stop := startTestServer(t, store.New(0))
	defer stop()
	c := dial(t, addr)
	defer c.close()

	c.send("set a 0 0 1\r\nA\r\n")
	c.expect("STORED")
	c.send("set c 0 0 1\r\nC\r\n")
	c.expect("STORED")

	// b is missing; only a and c should produce VALUE lines.
	c.send("get a b c\r\n")
	c.expect("VALUE a 0 1")
	c.expect("A")
	c.expect("VALUE c 0 1")
	c.expect("C")
	c.expect("END")
}

func TestIntegrationCAS(t *testing.T) {
	addr, stop := startTestServer(t, store.New(0))
	defer stop()
	c := dial(t, addr)
	defer c.close()

	c.send("set k 0 0 1\r\n1\r\n")
	c.expect("STORED")
	c.send("gets k\r\n")
	head := c.line() // VALUE k 0 1 <cas>
	c.expect("1")
	c.expect("END")
	parts := strings.Fields(head)
	casTok := parts[len(parts)-1]

	// Wrong token -> EXISTS.
	c.send("cas k 0 0 1 999999\r\n2\r\n")
	c.expect("EXISTS")

	// Correct token -> STORED.
	c.send("cas k 0 0 1 " + casTok + "\r\n2\r\n")
	c.expect("STORED")

	// cas on missing key -> NOT_FOUND.
	c.send("cas nope 0 0 1 1\r\nx\r\n")
	c.expect("NOT_FOUND")
}

func TestIntegrationErrors(t *testing.T) {
	addr, stop := startTestServer(t, store.New(0))
	defer stop()
	c := dial(t, addr)
	defer c.close()

	c.send("frobnicate\r\n")
	c.expect("ERROR")

	c.send("set\r\n")
	c.expect("CLIENT_ERROR bad command line format")

	// Bad data chunk: declared 3 bytes but terminator misplaced.
	c.send("set k 0 0 3\r\ntoolong\r\n")
	c.expect("CLIENT_ERROR bad data chunk")

	// Connection still usable.
	c.send("set k 0 0 2\r\nhi\r\n")
	c.expect("STORED")
}

func TestIntegrationNoReply(t *testing.T) {
	addr, stop := startTestServer(t, store.New(0))
	defer stop()
	c := dial(t, addr)
	defer c.close()

	// noreply suppresses STORED; the following get proves it took effect and
	// that no stray STORED line is sitting in the stream.
	c.send("set k 0 0 1 noreply\r\nZ\r\n")
	c.send("get k\r\n")
	c.expect("VALUE k 0 1")
	c.expect("Z")
	c.expect("END")
}

func TestIntegrationExpiry(t *testing.T) {
	// Inject a clock so we don't actually sleep.
	now := int64(1000)
	var mu sync.Mutex
	st := store.NewWithClock(0, func() int64 {
		mu.Lock()
		defer mu.Unlock()
		return now
	})
	addr, stop := startTestServer(t, st)
	defer stop()
	c := dial(t, addr)
	defer c.close()

	c.send("set k 0 1 1\r\nV\r\n")
	c.expect("STORED")
	c.send("get k\r\n")
	c.expect("VALUE k 0 1")
	c.expect("V")
	c.expect("END")

	mu.Lock()
	now = 1001 // advance past expiry
	mu.Unlock()

	c.send("get k\r\n")
	c.expect("END") // expired, no VALUE
}

func TestIntegrationPipelining(t *testing.T) {
	addr, stop := startTestServer(t, store.New(0))
	defer stop()
	c := dial(t, addr)
	defer c.close()

	// Three commands in one write.
	c.send("set a 0 0 1\r\nA\r\nset b 0 0 1\r\nB\r\nget a b\r\n")
	c.expect("STORED")
	c.expect("STORED")
	c.expect("VALUE a 0 1")
	c.expect("A")
	c.expect("VALUE b 0 1")
	c.expect("B")
	c.expect("END")
}

func TestIntegrationConcurrentClients(t *testing.T) {
	addr, stop := startTestServer(t, store.New(0))
	defer stop()

	const clients = 20
	var wg sync.WaitGroup
	for i := 0; i < clients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			c := dial(t, addr)
			defer c.close()
			key := "k" + string(rune('A'+id))
			for j := 0; j < 50; j++ {
				c.send("set " + key + " 0 0 1\r\nV\r\n")
				c.expect("STORED")
				c.send("get " + key + "\r\n")
				c.expect("VALUE " + key + " 0 1")
				c.expect("V")
				c.expect("END")
			}
		}(i)
	}
	wg.Wait()
}
