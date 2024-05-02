package main

import (
	"bytes"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// dialBack starts a loopback TCP listener and returns its address plus a
// channel that yields the single accepted server-side conn.
func loopbackPair(t *testing.T) (client net.Conn, server net.Conn) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	type res struct {
		c   net.Conn
		err error
	}
	ch := make(chan res, 1)
	go func() {
		c, err := ln.Accept()
		ch <- res{c, err}
	}()

	client, err = net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	r := <-ch
	if r.err != nil {
		t.Fatalf("accept: %v", r.err)
	}
	return client, r.c
}

// TestRelayEcho drives the relay against a loopback echo server: the client
// sends a payload via its (EOF-terminated) stdin, half-closes, and must read
// the same bytes echoed back into its stdout.
func TestRelayEcho(t *testing.T) {
	client, server := loopbackPair(t)
	defer client.Close()

	go func() {
		io.Copy(server, server) // echo
		server.Close()
	}()

	payload := []byte("hello, netcat\n")
	in := bytes.NewReader(payload)
	var out bytes.Buffer

	done := make(chan error, 1)
	go func() { done <- relay(client, in, &out, true) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("relay: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("relay timed out")
	}

	if !bytes.Equal(out.Bytes(), payload) {
		t.Fatalf("echo mismatch: got %q want %q", out.Bytes(), payload)
	}
}

// TestRelayBidirectional runs two relays back-to-back over loopback (a full
// client <-> server data path) and asserts byte-exact delivery in BOTH
// directions. Because each relay's conn->out copy only returns at peer EOF
// (the peer's CloseWrite), all peer data is guaranteed delivered before return.
func TestRelayBidirectional(t *testing.T) {
	client, server := loopbackPair(t)
	defer client.Close()
	defer server.Close()

	clientIn := []byte("ping from client")
	serverIn := []byte("pong from server")
	var clientOut, serverOut bytes.Buffer

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		relay(server, bytes.NewReader(serverIn), &serverOut, true)
	}()
	go func() {
		defer wg.Done()
		relay(client, bytes.NewReader(clientIn), &clientOut, true)
	}()

	waitTimeout(t, &wg, 5*time.Second)

	if !bytes.Equal(serverOut.Bytes(), clientIn) {
		t.Fatalf("client->server: got %q want %q", serverOut.Bytes(), clientIn)
	}
	if !bytes.Equal(clientOut.Bytes(), serverIn) {
		t.Fatalf("server->client: got %q want %q", clientOut.Bytes(), serverIn)
	}
}

// TestRelayHalfClose asserts that local stdin EOF half-closes the connection so
// the peer observes EOF, yet a reply already queued by the peer still reaches
// our stdout before the relay exits.
func TestRelayHalfClose(t *testing.T) {
	client, server := loopbackPair(t)
	defer client.Close()

	peerSawEOF := make(chan []byte, 1)
	go func() {
		// Read everything the client sends until EOF (its half-close).
		got, _ := io.ReadAll(server)
		// Then send a reply and close, so client's conn->out completes.
		server.Write([]byte("reply"))
		server.Close()
		peerSawEOF <- got
	}()

	in := bytes.NewReader([]byte("request"))
	var out bytes.Buffer
	if err := relay(client, in, &out, true); err != nil {
		t.Fatalf("relay: %v", err)
	}

	got := <-peerSawEOF
	if string(got) != "request" {
		t.Fatalf("peer received %q, want %q", got, "request")
	}
	if out.String() != "reply" {
		t.Fatalf("client out %q, want %q", out.String(), "reply")
	}
}

func waitTimeout(t *testing.T, wg *sync.WaitGroup, d time.Duration) {
	t.Helper()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatal("timed out waiting for relays")
	}
}
