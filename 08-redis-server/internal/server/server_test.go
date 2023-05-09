package server

import (
	"bufio"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"rediserver/internal/store"
)

// startTestServer launches a server on an ephemeral port and returns its
// address plus a cleanup func.
func startTestServer(t *testing.T) (string, *Server) {
	t.Helper()
	st := store.New()
	srv := New(st, "", log.New(io.Discard, "", 0))
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go srv.Serve(ln)
	t.Cleanup(func() { ln.Close() })
	return ln.Addr().String(), srv
}

// conn is a tiny RESP test client.
type conn struct {
	c net.Conn
	r *bufio.Reader
}

func dial(t *testing.T, addr string) *conn {
	t.Helper()
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return &conn{c: c, r: bufio.NewReader(c)}
}

// cmd sends a command as a RESP array of bulk strings and returns the raw
// reply bytes (read by type so we consume exactly one reply).
func (cn *conn) cmd(t *testing.T, args ...string) string {
	t.Helper()
	var b strings.Builder
	b.WriteString("*")
	b.WriteString(itoa(len(args)))
	b.WriteString("\r\n")
	for _, a := range args {
		b.WriteString("$")
		b.WriteString(itoa(len(a)))
		b.WriteString("\r\n")
		b.WriteString(a)
		b.WriteString("\r\n")
	}
	cn.c.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := cn.c.Write([]byte(b.String())); err != nil {
		t.Fatalf("write: %v", err)
	}
	cn.c.SetReadDeadline(time.Now().Add(2 * time.Second))
	return cn.readReply(t)
}

func (cn *conn) readReply(t *testing.T) string {
	t.Helper()
	line, err := cn.r.ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	switch line[0] {
	case '+', '-', ':':
		return line
	case '$':
		n := atoi(strings.TrimRight(line[1:], "\r\n"))
		if n == -1 {
			return line
		}
		buf := make([]byte, n+2)
		io.ReadFull(cn.r, buf)
		return line + string(buf)
	case '*':
		n := atoi(strings.TrimRight(line[1:], "\r\n"))
		out := line
		for i := 0; i < n; i++ {
			out += cn.readReply(t)
		}
		return out
	}
	return line
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

func atoi(s string) int {
	n, neg := 0, false
	for i, c := range s {
		if i == 0 && c == '-' {
			neg = true
			continue
		}
		n = n*10 + int(c-'0')
	}
	if neg {
		return -n
	}
	return n
}

func TestServerCommands(t *testing.T) {
	addr, _ := startTestServer(t)
	cn := dial(t, addr)

	cases := []struct {
		args []string
		want string
	}{
		{[]string{"PING"}, "+PONG\r\n"},
		{[]string{"PING", "hello"}, "$5\r\nhello\r\n"},
		{[]string{"ECHO", "hi there"}, "$8\r\nhi there\r\n"},
		{[]string{"SET", "name", "claude"}, "+OK\r\n"},
		{[]string{"GET", "name"}, "$6\r\nclaude\r\n"},
		{[]string{"GET", "missing"}, "$-1\r\n"},
		{[]string{"EXISTS", "name", "missing"}, ":1\r\n"},
		{[]string{"DEL", "name"}, ":1\r\n"},
		{[]string{"GET", "name"}, "$-1\r\n"},
		{[]string{"INCR", "counter"}, ":1\r\n"},
		{[]string{"INCR", "counter"}, ":2\r\n"},
		{[]string{"DECR", "counter"}, ":1\r\n"},
		{[]string{"GET", "counter"}, "$1\r\n1\r\n"},
		{[]string{"RPUSH", "mylist", "a", "b", "c"}, ":3\r\n"},
		{[]string{"LPUSH", "mylist", "z"}, ":4\r\n"},
		{[]string{"LRANGE", "mylist", "0", "-1"}, "*4\r\n$1\r\nz\r\n$1\r\na\r\n$1\r\nb\r\n$1\r\nc\r\n"},
		{[]string{"NOPE"}, "-ERR unknown command 'NOPE'\r\n"},
		{[]string{"SET"}, "-ERR wrong number of arguments for 'set' command\r\n"},
		{[]string{"INCR", "mylist"}, "-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"},
		{[]string{"COMMAND", "DOCS"}, "*0\r\n"},
	}
	for _, c := range cases {
		got := cn.cmd(t, c.args...)
		if got != c.want {
			t.Errorf("%v => %q, want %q", c.args, got, c.want)
		}
	}
}

func TestServerExpiry(t *testing.T) {
	addr, _ := startTestServer(t)
	cn := dial(t, addr)

	if got := cn.cmd(t, "SET", "tmp", "x", "PX", "50"); got != "+OK\r\n" {
		t.Fatalf("SET PX => %q", got)
	}
	if got := cn.cmd(t, "GET", "tmp"); got != "$1\r\nx\r\n" {
		t.Errorf("GET before expiry => %q, want $1\\r\\nx", got)
	}
	time.Sleep(120 * time.Millisecond)
	if got := cn.cmd(t, "GET", "tmp"); got != "$-1\r\n" {
		t.Errorf("GET after expiry => %q, want $-1", got)
	}

	// EX 0 is invalid.
	if got := cn.cmd(t, "SET", "k", "v", "EX", "0"); !strings.HasPrefix(got, "-ERR invalid expire time") {
		t.Errorf("SET EX 0 => %q", got)
	}
	// Both NX and XX is a syntax error.
	if got := cn.cmd(t, "SET", "k", "v", "NX", "XX"); got != "-ERR syntax error\r\n" {
		t.Errorf("SET NX XX => %q", got)
	}
}

func TestServerSetGetOption(t *testing.T) {
	addr, _ := startTestServer(t)
	cn := dial(t, addr)
	cn.cmd(t, "SET", "k", "old")
	if got := cn.cmd(t, "SET", "k", "new", "GET"); got != "$3\r\nold\r\n" {
		t.Errorf("SET ... GET => %q, want old", got)
	}
	if got := cn.cmd(t, "GET", "k"); got != "$3\r\nnew\r\n" {
		t.Errorf("GET after SET GET => %q", got)
	}
}

// TestServerConcurrent hammers the server from many connections concurrently
// and asserts INCR loses no updates.
func TestServerConcurrent(t *testing.T) {
	addr, srv := startTestServer(t)
	const (
		clients = 25
		perC    = 200
	)
	var wg sync.WaitGroup
	wg.Add(clients)
	for i := 0; i < clients; i++ {
		go func() {
			defer wg.Done()
			cn := &conn{}
			c, err := net.Dial("tcp", addr)
			if err != nil {
				t.Errorf("dial: %v", err)
				return
			}
			defer c.Close()
			cn.c = c
			cn.r = bufio.NewReader(c)
			for j := 0; j < perC; j++ {
				cn.cmd(t, "INCR", "shared")
			}
		}()
	}
	wg.Wait()

	got, _, _ := srv.Store.Get("shared")
	if got != "5000" {
		t.Errorf("final shared counter = %q, want 5000", got)
	}
}
