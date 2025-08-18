package whois

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

// fakeServer starts an in-process port-43-style server on an ephemeral port.
// It reads one CRLF-terminated request, passes it to handler, writes the
// returned body, then closes the connection (EOF) to end the response. The
// last request received is recorded for assertions.
type fakeServer struct {
	ln       net.Listener
	addr     string
	mu       chan string // buffered, holds last request line
	handler  func(request string) string
	holdOpen bool // if true, never write/close (to exercise read deadline)
}

func newFakeServer(t *testing.T, handler func(string) string) *fakeServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	fs := &fakeServer{
		ln:      ln,
		addr:    ln.Addr().String(),
		mu:      make(chan string, 8),
		handler: handler,
	}
	go fs.serve()
	t.Cleanup(func() { ln.Close() })
	return fs
}

func (fs *fakeServer) serve() {
	for {
		conn, err := fs.ln.Accept()
		if err != nil {
			return
		}
		go func(c net.Conn) {
			defer c.Close()
			r := bufio.NewReader(c)
			line, _ := r.ReadString('\n')
			req := strings.TrimRight(line, "\r\n")
			select {
			case fs.mu <- req:
			default:
			}
			if fs.holdOpen {
				time.Sleep(2 * time.Second)
				return
			}
			_, _ = c.Write([]byte(fs.handler(req)))
		}(conn)
	}
}

func (fs *fakeServer) lastRequest(t *testing.T) string {
	t.Helper()
	select {
	case req := <-fs.mu:
		return req
	case <-time.After(time.Second):
		t.Fatal("server received no request")
		return ""
	}
}

func TestClientQueryRoundTrip(t *testing.T) {
	body := "Domain Name: EXAMPLE.COM\nRegistrar: Example Registrar, LLC\n"
	fs := newFakeServer(t, func(req string) string { return body })

	c := NewClient()
	got, err := c.Query(context.Background(), fs.addr, "domain example.com")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if got != body {
		t.Errorf("body = %q, want %q", got, body)
	}
	// Verify the query builder + CRLF framing reached the wire intact.
	if req := fs.lastRequest(t); req != "domain example.com" {
		t.Errorf("server saw request %q, want %q", req, "domain example.com")
	}
}

func TestClientQueryEmptyResponse(t *testing.T) {
	fs := newFakeServer(t, func(string) string { return "" })
	c := NewClient()
	got, err := c.Query(context.Background(), fs.addr, "example.com")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty response, got %q", got)
	}
}

func TestClientQueryReadDeadline(t *testing.T) {
	fs := newFakeServer(t, func(string) string { return "" })
	fs.holdOpen = true // accept but never respond/close

	c := NewClient()
	c.Timeout = 150 * time.Millisecond
	start := time.Now()
	_, err := c.Query(context.Background(), fs.addr, "example.com")
	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("deadline not honoured, took %v", elapsed)
	}
}

func TestClientQueryDialError(t *testing.T) {
	c := NewClient()
	c.Timeout = 200 * time.Millisecond
	// 127.0.0.1:1 is almost certainly refused.
	_, err := c.Query(context.Background(), "127.0.0.1:1", "example.com")
	if err == nil {
		t.Fatal("expected dial error, got nil")
	}
}
