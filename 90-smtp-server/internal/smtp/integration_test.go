package smtp

import (
	"bufio"
	"context"
	"fmt"
	"net"
	netsmtp "net/smtp"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

// startTestServer launches a real Server on an ephemeral loopback port backed by
// an in-memory store and returns the address, store, and a cleanup function.
func startTestServer(t *testing.T) (string, *MemoryStore, func()) {
	t.Helper()
	store := NewMemoryStore()
	srv := NewServer("test.local", store, 30*time.Second)
	if err := srv.Listen("127.0.0.1:0"); err != nil {
		t.Fatalf("listen: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = srv.Serve(ctx)
		close(done)
	}()
	addr := srv.Addr().String()
	cleanup := func() {
		cancel()
		<-done
	}
	return addr, store, cleanup
}

// client is a tiny line-oriented SMTP test client.
type client struct {
	conn net.Conn
	r    *bufio.Reader
}

func dial(t *testing.T, addr string) *client {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return &client{conn: conn, r: bufio.NewReader(conn)}
}

// readReply reads one reply (handling multiline) and returns the code and text.
func (c *client) readReply(t *testing.T) (int, string) {
	t.Helper()
	var lastCode int
	var text strings.Builder
	for {
		c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		line, err := c.r.ReadString('\n')
		if err != nil {
			t.Fatalf("read reply: %v (got %q)", err, line)
		}
		line = strings.TrimRight(line, "\r\n")
		if len(line) < 3 {
			t.Fatalf("short reply line: %q", line)
		}
		fmt.Sscanf(line[:3], "%d", &lastCode)
		text.WriteString(line[3:])
		// Multiline continuation uses '-' at position 3; final line uses ' '.
		if len(line) == 3 || line[3] == ' ' {
			break
		}
	}
	return lastCode, text.String()
}

func (c *client) cmd(t *testing.T, line string) (int, string) {
	t.Helper()
	c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := c.conn.Write([]byte(line + "\r\n")); err != nil {
		t.Fatalf("write %q: %v", line, err)
	}
	return c.readReply(t)
}

func (c *client) write(t *testing.T, data string) {
	t.Helper()
	c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if _, err := c.conn.Write([]byte(data)); err != nil {
		t.Fatalf("write data: %v", err)
	}
}

func (c *client) close() { c.conn.Close() }

func expect(t *testing.T, gotCode, wantCode int, ctx string) {
	t.Helper()
	if gotCode != wantCode {
		t.Fatalf("%s: got %d, want %d", ctx, gotCode, wantCode)
	}
}

func TestIntegrationFullSession(t *testing.T) {
	addr, store, cleanup := startTestServer(t)
	defer cleanup()

	c := dial(t, addr)
	defer c.close()

	code, _ := c.readReply(t)
	expect(t, code, 220, "greeting")

	code, _ = c.cmd(t, "HELO tester")
	expect(t, code, 250, "HELO")

	code, _ = c.cmd(t, "MAIL FROM:<s@ex.com>")
	expect(t, code, 250, "MAIL")

	code, _ = c.cmd(t, "RCPT TO:<r1@ex.com>")
	expect(t, code, 250, "RCPT1")

	code, _ = c.cmd(t, "RCPT TO:<r2@ex.com>")
	expect(t, code, 250, "RCPT2")

	code, _ = c.cmd(t, "DATA")
	expect(t, code, 354, "DATA")

	c.write(t, "Subject: Hello\r\n\r\nThis is the body.\r\n.\r\n")
	code, _ = c.readReply(t)
	expect(t, code, 250, "end of DATA")

	code, _ = c.cmd(t, "QUIT")
	expect(t, code, 221, "QUIT")

	if store.Count() != 1 {
		t.Fatalf("store has %d messages, want 1", store.Count())
	}
	msg := store.All()[0]
	if msg.From != "s@ex.com" {
		t.Errorf("From = %q", msg.From)
	}
	if len(msg.To) != 2 || msg.To[0] != "r1@ex.com" || msg.To[1] != "r2@ex.com" {
		t.Errorf("To = %v", msg.To)
	}
	if !strings.Contains(string(msg.Data), "This is the body.") {
		t.Errorf("body = %q", msg.Data)
	}
}

func TestIntegrationDotStuffing(t *testing.T) {
	addr, store, cleanup := startTestServer(t)
	defer cleanup()

	c := dial(t, addr)
	defer c.close()
	c.readReply(t)
	c.cmd(t, "HELO tester")
	c.cmd(t, "MAIL FROM:<s@ex.com>")
	c.cmd(t, "RCPT TO:<r@ex.com>")
	code, _ := c.cmd(t, "DATA")
	expect(t, code, 354, "DATA")

	// Body contains a stuffed ".." (-> ".") and a line "..." (-> "..").
	// A lone "." mid-body would terminate, so it is sent stuffed as "..".
	c.write(t, "line one\r\n..\r\n...\r\nlast line\r\n.\r\n")
	code, _ = c.readReply(t)
	expect(t, code, 250, "end of DATA")

	if store.Count() != 1 {
		t.Fatalf("store has %d messages, want 1 (terminated early?)", store.Count())
	}
	body := string(store.All()[0].Data)
	want := "line one\r\n.\r\n..\r\nlast line\r\n"
	if body != want {
		t.Errorf("unstuffed body = %q, want %q", body, want)
	}
}

func TestIntegrationOutOfOrder(t *testing.T) {
	addr, _, cleanup := startTestServer(t)
	defer cleanup()

	c := dial(t, addr)
	defer c.close()
	c.readReply(t)

	code, _ := c.cmd(t, "MAIL FROM:<s@ex.com>")
	expect(t, code, 503, "MAIL before HELO")

	c.cmd(t, "HELO tester")

	code, _ = c.cmd(t, "DATA")
	expect(t, code, 503, "DATA before RCPT")

	code, _ = c.cmd(t, "RCPT TO:<r@ex.com>")
	expect(t, code, 503, "RCPT before MAIL")

	code, _ = c.cmd(t, "MAIL FROM:garbage")
	expect(t, code, 501, "malformed MAIL")
}

func TestIntegrationRset(t *testing.T) {
	addr, store, cleanup := startTestServer(t)
	defer cleanup()

	c := dial(t, addr)
	defer c.close()
	c.readReply(t)
	c.cmd(t, "HELO tester")
	c.cmd(t, "MAIL FROM:<s@ex.com>")
	c.cmd(t, "RCPT TO:<r@ex.com>")

	code, _ := c.cmd(t, "RSET")
	expect(t, code, 250, "RSET")

	code, _ = c.cmd(t, "DATA")
	expect(t, code, 503, "DATA after RSET")

	// New transaction must succeed afterward.
	c.cmd(t, "MAIL FROM:<s2@ex.com>")
	c.cmd(t, "RCPT TO:<r2@ex.com>")
	code, _ = c.cmd(t, "DATA")
	expect(t, code, 354, "DATA new txn")
	c.write(t, "hi\r\n.\r\n")
	code, _ = c.readReply(t)
	expect(t, code, 250, "store new txn")

	if store.Count() != 1 {
		t.Fatalf("store has %d, want 1", store.Count())
	}
	if store.All()[0].From != "s2@ex.com" {
		t.Errorf("From = %q, want s2@ex.com", store.All()[0].From)
	}
}

func TestIntegrationConcurrency(t *testing.T) {
	addr, store, cleanup := startTestServer(t)
	defer cleanup()

	const n = 20
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c := dial(t, addr)
			defer c.close()
			c.readReply(t)
			c.cmd(t, "HELO tester")
			c.cmd(t, fmt.Sprintf("MAIL FROM:<s%d@ex.com>", i))
			c.cmd(t, fmt.Sprintf("RCPT TO:<r%d@ex.com>", i))
			c.cmd(t, "DATA")
			c.write(t, fmt.Sprintf("Subject: msg %d\r\n\r\nbody %d\r\n.\r\n", i, i))
			code, _ := c.readReply(t)
			expect(t, code, 250, "concurrent store")
			c.cmd(t, "QUIT")
		}(i)
	}
	wg.Wait()

	if store.Count() != n {
		t.Fatalf("store has %d messages, want %d", store.Count(), n)
	}
	// Each message's body must match its own sender (no interleaving).
	for _, m := range store.All() {
		sender := strings.TrimSuffix(strings.TrimPrefix(m.From, "s"), "@ex.com")
		if !strings.Contains(string(m.Data), "body "+sender) {
			t.Errorf("message From=%s has mismatched body %q", m.From, m.Data)
		}
	}
}

// TestIntegrationNetSMTPClient drives the server with Go's standard net/smtp
// client, proving interoperability with an independent SMTP implementation.
func TestIntegrationNetSMTPClient(t *testing.T) {
	addr, store, cleanup := startTestServer(t)
	defer cleanup()

	c, err := netsmtp.Dial(addr)
	if err != nil {
		t.Fatalf("smtp.Dial: %v", err)
	}
	defer c.Close()

	if err := c.Hello("tester"); err != nil {
		t.Fatalf("HELO: %v", err)
	}
	if err := c.Mail("from@ex.com"); err != nil {
		t.Fatalf("MAIL: %v", err)
	}
	if err := c.Rcpt("to@ex.com"); err != nil {
		t.Fatalf("RCPT: %v", err)
	}
	wc, err := c.Data()
	if err != nil {
		t.Fatalf("DATA: %v", err)
	}
	body := "Subject: Hi\r\n\r\nHello CC SMTP\r\n"
	if _, err := wc.Write([]byte(body)); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := wc.Close(); err != nil {
		t.Fatalf("close data: %v", err)
	}
	if err := c.Quit(); err != nil {
		t.Fatalf("QUIT: %v", err)
	}

	if store.Count() != 1 {
		t.Fatalf("store has %d messages, want 1", store.Count())
	}
	m := store.All()[0]
	if m.From != "from@ex.com" || len(m.To) != 1 || m.To[0] != "to@ex.com" {
		t.Errorf("envelope = %+v", m)
	}
	if !strings.Contains(string(m.Data), "Hello CC SMTP") {
		t.Errorf("body = %q", m.Data)
	}
}

// TestIntegrationPythonSMTPLib shells out to Python's standard-library smtplib
// client. It is skipped when Python is not on PATH.
func TestIntegrationPythonSMTPLib(t *testing.T) {
	py := pythonExe()
	if py == "" {
		t.Skip("python not available")
	}
	addr, store, cleanup := startTestServer(t)
	defer cleanup()

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}

	script := fmt.Sprintf(`
import smtplib, sys
with smtplib.SMTP(%q, %s, timeout=5) as s:
    s.helo("pytester")
    s.sendmail("py@ex.com", ["rcpt@ex.com"], "Subject: From Python\r\n\r\nhello from smtplib")
print("ok")
`, host, port)

	cmd := exec.Command(py, "-c", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("python smtplib failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "ok") {
		t.Fatalf("unexpected python output: %s", out)
	}

	if store.Count() != 1 {
		t.Fatalf("store has %d messages, want 1", store.Count())
	}
	m := store.All()[0]
	if m.From != "py@ex.com" || len(m.To) != 1 || m.To[0] != "rcpt@ex.com" {
		t.Errorf("envelope = %+v", m)
	}
	if !strings.Contains(string(m.Data), "hello from smtplib") {
		t.Errorf("body = %q", m.Data)
	}
}

func pythonExe() string {
	for _, name := range []string{"python3", "python"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	if _, err := os.Stat("/usr/bin/python3"); err == nil {
		return "/usr/bin/python3"
	}
	return ""
}
