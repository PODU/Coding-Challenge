package relay_test

import (
	"bytes"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"ccsocat/internal/endpoint"
	"ccsocat/internal/relay"
)

// pipeEnd is a minimal in-memory ReadWriteCloser built from a separate reader
// and writer, used to inject known streams into the relay.
type pipeEnd struct {
	io.Reader
	io.Writer
}

func (pipeEnd) Close() error { return nil }

// readN reads exactly n bytes (or fails the test) within a deadline.
func readN(t *testing.T, r net.Conn, n int) []byte {
	t.Helper()
	_ = r.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		t.Fatalf("readN: %v", err)
	}
	_ = r.SetReadDeadline(time.Time{})
	return buf
}

func await(t *testing.T, ch <-chan error) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(3 * time.Second):
		t.Fatal("relay did not finish in time (possible deadlock)")
		return nil
	}
}

// TestBidirectionalLoopback bridges two net.Pipe "wires" and verifies data
// flows in both directions, mirroring a TCP<->TCP bridge with no real sockets.
func TestBidirectionalLoopback(t *testing.T) {
	a1, a2 := net.Pipe() // wire A: a1 is the external peer, a2 feeds the relay
	b1, b2 := net.Pipe() // wire B: b1 feeds the relay, b2 is the external peer

	done := make(chan error, 1)
	go func() { done <- relay.Relay(a2, b1, relay.Bidirectional) }()

	// left -> right: bytes written to a1 should surface at b2.
	go func() { _, _ = a1.Write([]byte("ping")) }()
	if got := string(readN(t, b2, 4)); got != "ping" {
		t.Fatalf("L->R got %q, want ping", got)
	}

	// right -> left: bytes written to b2 should surface at a1.
	go func() { _, _ = b2.Write([]byte("pong")) }()
	if got := string(readN(t, a1, 4)); got != "pong" {
		t.Fatalf("R->L got %q, want pong", got)
	}

	// Closing both external ends drives EOF in both directions.
	a1.Close()
	b2.Close()
	if err := await(t, done); err != nil {
		t.Fatalf("Relay returned %v, want nil", err)
	}
}

// TestStdioToTCP pairs an injected STDIO endpoint (in-memory buffers) with a
// net.Pipe "TCP" end and asserts the composite endpoint interoperates with the
// relay in both directions.
func TestStdioToTCP(t *testing.T) {
	stdin := bytes.NewBufferString("from-stdin")
	var stdout bytes.Buffer
	stdioEnd := endpoint.NewStdio(stdin, &stdout)

	tcpEnd, peer := net.Pipe()

	done := make(chan error, 1)
	go func() { done <- relay.Relay(stdioEnd, tcpEnd, relay.Bidirectional) }()

	// stdin bytes must reach the TCP peer.
	if got := string(readN(t, peer, len("from-stdin"))); got != "from-stdin" {
		t.Fatalf("stdin->tcp got %q", got)
	}

	// TCP bytes must reach the stdout buffer.
	go func() {
		_, _ = peer.Write([]byte("to-stdout"))
		peer.Close()
	}()
	if err := await(t, done); err != nil {
		t.Fatalf("Relay returned %v", err)
	}
	if got := stdout.String(); got != "to-stdout" {
		t.Fatalf("tcp->stdout got %q, want to-stdout", got)
	}
}

// TestUnidirectional verifies -u semantics: data flows left->right only and the
// reverse direction carries nothing.
func TestUnidirectional(t *testing.T) {
	var leftOut, rightOut bytes.Buffer
	// left reads "hello" then EOF; its writer captures any reverse traffic.
	left := pipeEnd{Reader: strings.NewReader("hello"), Writer: &leftOut}
	// right's reader would supply reverse data that must NOT be delivered.
	right := pipeEnd{Reader: strings.NewReader("SHOULD-NOT-APPEAR"), Writer: &rightOut}

	if err := relay.Relay(left, right, relay.UniLeftToRight); err != nil {
		t.Fatalf("Relay returned %v", err)
	}
	if rightOut.String() != "hello" {
		t.Fatalf("right got %q, want hello", rightOut.String())
	}
	if leftOut.Len() != 0 {
		t.Fatalf("left got %q, want nothing (unidirectional)", leftOut.String())
	}
}

// recCloser records half-close calls to prove FIN propagation on EOF.
type recCloser struct {
	src              *bytes.Reader
	written          bytes.Buffer
	closeWriteCalled bool
	closeReadCalled  bool
}

func (r *recCloser) Read(p []byte) (int, error)  { return r.src.Read(p) }
func (r *recCloser) Write(p []byte) (int, error) { return r.written.Write(p) }
func (r *recCloser) Close() error                { return nil }
func (r *recCloser) CloseWrite() error           { r.closeWriteCalled = true; return nil }
func (r *recCloser) CloseRead() error            { r.closeReadCalled = true; return nil }

// TestHalfCloseOnEOF verifies that when the left->right copy hits EOF the relay
// calls CloseWrite on the right endpoint (sending FIN downstream).
func TestHalfCloseOnEOF(t *testing.T) {
	var leftOut bytes.Buffer
	left := pipeEnd{Reader: strings.NewReader("payload"), Writer: &leftOut}
	right := &recCloser{src: bytes.NewReader(nil)} // reverse direction is at EOF

	if err := relay.Relay(left, right, relay.Bidirectional); err != nil {
		t.Fatalf("Relay returned %v", err)
	}
	if right.written.String() != "payload" {
		t.Fatalf("right received %q, want payload", right.written.String())
	}
	if !right.closeWriteCalled {
		t.Fatal("expected CloseWrite to be called on right at EOF")
	}
}
