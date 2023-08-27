package transport

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"

	"realtimechat/internal/chat"
)

// two raw-TCP clients; first line = name; a message from one appears as
// "Name: msg" on the other, not on the sender, as in the original challenge.
func TestTCPAdapter(t *testing.T) {
	hub := chat.NewHub()
	go hub.Run()
	t.Cleanup(hub.Shutdown)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go ServeTCP(ln, hub)

	connA := dialTCP(t, ln.Addr().String(), "alice")
	connB := dialTCP(t, ln.Addr().String(), "bob")

	// Let presence/system notices settle.
	time.Sleep(200 * time.Millisecond)

	if _, err := connA.conn.Write([]byte("hello there\n")); err != nil {
		t.Fatal(err)
	}

	// B should see "alice: hello there".
	connB.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if line := readLineContaining(t, connB.r, "alice: hello there", 2*time.Second); line == "" {
		t.Fatal("bob did not receive alice's message")
	}
	connB.conn.SetReadDeadline(time.Time{})

	// A should NOT see its own message echoed back.
	connA.conn.SetReadDeadline(time.Now().Add(400 * time.Millisecond))
	for {
		line, err := connA.r.ReadString('\n')
		if err != nil {
			break
		}
		if strings.Contains(line, "alice: hello there") {
			t.Fatalf("sender received its own message: %q", line)
		}
	}
}

type tcpClient struct {
	conn net.Conn
	r    *bufio.Reader
}

func dialTCP(t *testing.T, addr, name string) *tcpClient {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	r := bufio.NewReader(conn)
	// Consume the "Enter your name: " prompt (no trailing newline).
	conn.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 64)
	_, _ = r.Read(buf)
	conn.SetReadDeadline(time.Time{})
	if _, err := conn.Write([]byte(name + "\n")); err != nil {
		t.Fatal(err)
	}
	return &tcpClient{conn: conn, r: r}
}

func readLineContaining(t *testing.T, r *bufio.Reader, want string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		// Reading is bounded by the underlying conn deadline set by the caller.
		line, err := r.ReadString('\n')
		if strings.Contains(line, want) {
			return line
		}
		if err != nil {
			return ""
		}
	}
	return ""
}
