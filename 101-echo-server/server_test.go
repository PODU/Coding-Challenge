package ccecho

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"log/slog"
	"net"
	"runtime"
	"sync"
	"testing"
	"time"
)

// startServer launches a Server bound to 127.0.0.1:0 and returns its bound
// address plus a stop func that cancels the context and waits for Run to return.
func startServer(t *testing.T, cfg Config) (net.Addr, func()) {
	t.Helper()
	cfg.Host = "127.0.0.1"
	cfg.Port = 0
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() { runErr <- srv.Run(ctx) }()

	addrCtx, addrCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer addrCancel()
	addr := srv.Addr(addrCtx)
	if addr == nil {
		cancel()
		t.Fatalf("server did not start listening in time")
	}

	stop := func() {
		cancel()
		select {
		case err := <-runErr:
			if err != nil {
				t.Errorf("Run returned error: %v", err)
			}
		case <-time.After(15 * time.Second):
			t.Errorf("Run did not return after cancel")
		}
	}
	return addr, stop
}

func TestTCPSingleEcho(t *testing.T) {
	addr, stop := startServer(t, Config{Protocol: "tcp"})
	defer stop()

	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	payload := []byte("hello, echo server")
	if _, err := conn.Write(payload); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("echo mismatch: got %q want %q", got, payload)
	}
}

func TestTCPMultiWriteStream(t *testing.T) {
	addr, stop := startServer(t, Config{Protocol: "tcp"})
	defer stop()

	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	chunks := [][]byte{[]byte("alpha"), []byte("-beta-"), []byte("gamma!!")}
	var want []byte
	for _, c := range chunks {
		if _, err := conn.Write(c); err != nil {
			t.Fatalf("write: %v", err)
		}
		want = append(want, c...)
	}

	got := make([]byte, len(want))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("stream echo mismatch: got %q want %q", got, want)
	}
}

func TestTCPLargePayload(t *testing.T) {
	addr, stop := startServer(t, Config{Protocol: "tcp"})
	defer stop()

	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	const size = 4 << 20 // 4 MiB
	payload := make([]byte, size)
	if _, err := rand.Read(payload); err != nil {
		t.Fatalf("rand: %v", err)
	}

	// Read concurrently with writing to avoid socket buffer deadlock.
	got := make([]byte, size)
	readErr := make(chan error, 1)
	go func() {
		_, err := io.ReadFull(conn, got)
		readErr <- err
	}()

	if _, err := conn.Write(payload); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := <-readErr; err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("large payload echo mismatch (len got=%d want=%d)", len(got), len(payload))
	}
}

func TestTCPEOF(t *testing.T) {
	addr, stop := startServer(t, Config{Protocol: "tcp"})
	defer stop()

	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	payload := []byte("final message before half-close")
	if _, err := conn.Write(payload); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Half-close the write side; server should echo everything then see EOF.
	if tc, ok := conn.(*net.TCPConn); ok {
		if err := tc.CloseWrite(); err != nil {
			t.Fatalf("CloseWrite: %v", err)
		}
	}

	got, err := io.ReadAll(conn)
	if err != nil {
		t.Fatalf("readall: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("EOF echo mismatch: got %q want %q", got, payload)
	}
	conn.Close()
}

func TestTCPConcurrentClients(t *testing.T) {
	addr, stop := startServer(t, Config{Protocol: "tcp"})
	defer stop()

	const clients = 100
	var wg sync.WaitGroup
	errCh := make(chan error, clients)

	for i := 0; i < clients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			conn, err := net.Dial("tcp", addr.String())
			if err != nil {
				errCh <- err
				return
			}
			defer conn.Close()

			payload := []byte("client-" + string(rune('A'+id%26)) + "-payload-" + time.Now().Format("150405.000000000"))
			// Make each payload unique per id.
			payload = append(payload, byte(id), byte(id>>8))

			if _, err := conn.Write(payload); err != nil {
				errCh <- err
				return
			}
			got := make([]byte, len(payload))
			conn.SetReadDeadline(time.Now().Add(10 * time.Second))
			if _, err := io.ReadFull(conn, got); err != nil {
				errCh <- err
				return
			}
			if !bytes.Equal(got, payload) {
				errCh <- &mismatchError{id: id, got: got, want: payload}
				return
			}
		}(i)
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("timeout waiting for concurrent clients (possible deadlock)")
	}
	close(errCh)
	for err := range errCh {
		t.Errorf("client error: %v", err)
	}
}

type mismatchError struct {
	id        int
	got, want []byte
}

func (e *mismatchError) Error() string {
	return "client " + string(rune('0'+e.id%10)) + " got wrong echo"
}

func TestUDPDatagramEcho(t *testing.T) {
	addr, stop := startServer(t, Config{Protocol: "udp"})
	defer stop()

	conn, err := net.Dial("udp", addr.String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	payload := []byte("udp echo datagram")
	if _, err := conn.Write(payload); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := make([]byte, len(payload)+16)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := conn.Read(got)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got[:n], payload) {
		t.Fatalf("udp echo mismatch: got %q want %q", got[:n], payload)
	}
}

func TestUDPMultipleDatagramsAndSenders(t *testing.T) {
	addr, stop := startServer(t, Config{Protocol: "udp"})
	defer stop()

	const senders = 5
	const perSender = 4
	var wg sync.WaitGroup
	errCh := make(chan error, senders)

	for s := 0; s < senders; s++ {
		wg.Add(1)
		go func(sid int) {
			defer wg.Done()
			conn, err := net.Dial("udp", addr.String())
			if err != nil {
				errCh <- err
				return
			}
			defer conn.Close()
			for d := 0; d < perSender; d++ {
				payload := []byte{byte(sid), byte(d), 0xAA, 0xBB}
				if _, err := conn.Write(payload); err != nil {
					errCh <- err
					return
				}
				buf := make([]byte, 64)
				conn.SetReadDeadline(time.Now().Add(5 * time.Second))
				n, err := conn.Read(buf)
				if err != nil {
					errCh <- err
					return
				}
				if !bytes.Equal(buf[:n], payload) {
					errCh <- &mismatchError{id: sid, got: buf[:n], want: payload}
					return
				}
			}
		}(s)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("udp sender error: %v", err)
	}
}

func TestUDPEmptyDatagram(t *testing.T) {
	// The server echoes zero-length datagrams.
	// However, the Windows UDP stack does not deliver zero-length datagrams to
	// the receiver at all (the sender's Write returns 0/no-error but ReadFrom
	// never fires), so this round-trip cannot be exercised on that platform.
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not deliver zero-length UDP datagrams; server-side echo of n==0 is still correct")
	}
	addr, stop := startServer(t, Config{Protocol: "udp"})
	defer stop()

	conn, err := net.Dial("udp", addr.String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte{}); err != nil {
		t.Fatalf("write empty: %v", err)
	}
	buf := make([]byte, 16)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read empty: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected zero-length echo, got %d bytes", n)
	}
}

func TestGracefulDrainOnCancel(t *testing.T) {
	cfg := Config{Protocol: "tcp", DrainTimeout: 5 * time.Second}
	cfg.Host = "127.0.0.1"
	cfg.Port = 0
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))

	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() { runErr <- srv.Run(ctx) }()

	addrCtx, addrCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer addrCancel()
	addr := srv.Addr(addrCtx)
	if addr == nil {
		cancel()
		t.Fatal("server never listened")
	}

	conn, err := net.Dial("tcp", addr.String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send data, then cancel mid-flight; the in-flight echo must still arrive.
	payload := []byte("in-flight payload during shutdown")
	if _, err := conn.Write(payload); err != nil {
		t.Fatalf("write: %v", err)
	}
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("read echo: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("in-flight echo mismatch")
	}

	// Cancel: server stops accepting and drains.
	cancel()

	// New dials should now fail (listener closed).
	deadline := time.Now().Add(3 * time.Second)
	for {
		c2, err := net.Dial("tcp", addr.String())
		if err != nil {
			break
		}
		c2.Close()
		if time.Now().After(deadline) {
			t.Fatal("listener still accepting after shutdown")
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Existing connection: close our write side so the handler's io.Copy ends,
	// allowing drain to complete.
	if tc, ok := conn.(*net.TCPConn); ok {
		tc.CloseWrite()
	}

	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return within drain window")
	}
}

func TestShutdownNoClients(t *testing.T) {
	cfg := Config{Protocol: "tcp"}
	cfg.Host = "127.0.0.1"
	cfg.Port = 0
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))

	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() { runErr <- srv.Run(ctx) }()

	addrCtx, addrCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer addrCancel()
	if srv.Addr(addrCtx) == nil {
		cancel()
		t.Fatal("server never listened")
	}

	cancel()
	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return promptly with no clients")
	}
}

func TestInvalidPort(t *testing.T) {
	for _, p := range []int{-1, 70000, 99999} {
		if _, err := NewServer(Config{Protocol: "tcp", Port: p}); err == nil {
			t.Errorf("expected error for port %d, got nil", p)
		}
	}
}

func TestInvalidProtocol(t *testing.T) {
	if _, err := NewServer(Config{Protocol: "sctp", Port: 7000}); err == nil {
		t.Error("expected error for invalid protocol")
	}
}

func TestBindFailure(t *testing.T) {
	// Occupy a port, then try to bind a second server to it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("pre-bind: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	cfg := Config{
		Protocol: "tcp",
		Host:     "127.0.0.1",
		Port:     port,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := srv.Run(ctx); err == nil {
		t.Fatal("expected bind failure error, got nil")
	}
}
