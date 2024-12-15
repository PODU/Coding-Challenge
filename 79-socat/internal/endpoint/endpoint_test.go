package endpoint

import (
	"bytes"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"

	"ccsocat/internal/addr"
)

// TestMain lets the test binary impersonate a "cat" process (copy stdin to
// stdout) when CCSOCAT_TEST_CAT is set, giving the EXEC test a portable child
// program on every platform.
func TestMain(m *testing.M) {
	if os.Getenv("CCSOCAT_TEST_CAT") == "1" {
		_, _ = io.Copy(os.Stdout, os.Stdin)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// TestFileAppend exercises the FILE endpoint with create+append: two separate
// opens should append rather than truncate.
func TestFileAppend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.txt")

	spec := &addr.AddressSpec{
		Kind:    addr.KindFile,
		Params:  []string{path},
		Options: map[string]string{"create": "", "append": ""},
	}

	write := func(data string) {
		ep, err := Open(spec, RoleRight)
		if err != nil {
			t.Fatalf("Open FILE: %v", err)
		}
		if _, err := io.Copy(ep, bytes.NewBufferString(data)); err != nil {
			t.Fatalf("write: %v", err)
		}
		if err := ep.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	}

	write("line1\n")
	write("line2\n")

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "line1\nline2\n" {
		t.Fatalf("file = %q, want appended content", string(got))
	}
}

// TestFileRead opens an existing file read-only as the left (source) role.
func TestFileRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "in.txt")
	if err := os.WriteFile(path, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	spec := &addr.AddressSpec{Kind: addr.KindFile, Params: []string{path}, Options: map[string]string{}}
	ep, err := Open(spec, RoleLeft)
	if err != nil {
		t.Fatal(err)
	}
	defer ep.Close()
	got, _ := io.ReadAll(ep)
	if string(got) != "payload" {
		t.Fatalf("read %q, want payload", string(got))
	}
}

// TestStdioEndpoint checks the composite STDIO endpoint over injected buffers.
func TestStdioEndpoint(t *testing.T) {
	in := bytes.NewBufferString("hello")
	var out bytes.Buffer
	ep := NewStdio(in, &out)

	buf := make([]byte, 5)
	n, err := ep.Read(buf)
	if err != nil || string(buf[:n]) != "hello" {
		t.Fatalf("Read = %q, %v", string(buf[:n]), err)
	}
	if _, err := ep.Write([]byte("world")); err != nil {
		t.Fatal(err)
	}
	if out.String() != "world" {
		t.Fatalf("Write captured %q, want world", out.String())
	}
	if err := ep.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestExecEcho spawns a child process and round-trips bytes through its stdio.
func TestExecEcho(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping EXEC test in -short mode")
	}
	t.Setenv("CCSOCAT_TEST_CAT", "1")

	ep, err := newExecArgv([]string{os.Args[0]})
	if err != nil {
		t.Fatalf("newExecArgv: %v", err)
	}

	if _, err := io.WriteString(ep, "echo-me"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := ep.CloseWrite(); err != nil {
		t.Fatalf("CloseWrite: %v", err)
	}
	got, err := io.ReadAll(ep)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := ep.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if string(got) != "echo-me" {
		t.Fatalf("EXEC round trip = %q, want echo-me", string(got))
	}
}

// TestTCPListenLoopback covers the TCP-LISTEN factory (with reuseaddr) and a
// real loopback connection on an ephemeral port.
func TestTCPListenLoopback(t *testing.T) {
	spec, err := addr.Parse("TCP4-LISTEN:0,reuseaddr")
	if err != nil {
		t.Fatal(err)
	}
	ln, err := Listen(spec)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	serverGot := make(chan string, 1)
	go func() {
		ep, err := ln.Accept()
		if err != nil {
			serverGot <- "ACCEPT-ERR:" + err.Error()
			return
		}
		defer ep.Close()
		b, _ := io.ReadAll(ep)
		serverGot <- string(b)
	}()

	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	c, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", port))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if _, err := c.Write([]byte("ten messages")); err != nil {
		t.Fatal(err)
	}
	// Half-close so the server's ReadAll observes EOF.
	if tcp, ok := c.(*net.TCPConn); ok {
		_ = tcp.CloseWrite()
	}

	if got := <-serverGot; got != "ten messages" {
		t.Fatalf("server received %q, want 'ten messages'", got)
	}
	c.Close()
}
