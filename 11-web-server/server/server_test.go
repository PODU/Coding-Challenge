package server

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// startTestServer spins up a Server on an ephemeral port serving a temp root
// and returns the base URL plus its raw host:port. It tears down on cleanup.
func startTestServer(t *testing.T) (baseURL, addr string) {
	t.Helper()
	root := newTestRoot(t)

	srv := &Server{addr: "127.0.0.1:0", root: root}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = srv.Serve(ctx, ln)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("server did not shut down")
		}
	})

	return "http://" + ln.Addr().String(), ln.Addr().String()
}

func TestIntegration_ServeIndex(t *testing.T) {
	base, _ := startTestServer(t)

	resp, err := http.Get(base + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("content-type = %q", ct)
	}
	if resp.Header.Get("Server") == "" || resp.Header.Get("Date") == "" {
		t.Fatal("missing Server/Date header")
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "home") {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestIntegration_ContentTypeCSS(t *testing.T) {
	base, _ := startTestServer(t)
	resp, err := http.Get(base + "/style.css")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/css; charset=utf-8" {
		t.Fatalf("content-type = %q", ct)
	}
}

func TestIntegration_NotFound(t *testing.T) {
	base, _ := startTestServer(t)
	resp, err := http.Get(base + "/missing.html")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestIntegration_MethodNotAllowed(t *testing.T) {
	base, _ := startTestServer(t)
	resp, err := http.Post(base+"/", "text/plain", strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", resp.StatusCode)
	}
	if resp.Header.Get("Allow") != "GET, HEAD" {
		t.Fatalf("Allow = %q", resp.Header.Get("Allow"))
	}
}

func TestIntegration_HeadNoBody(t *testing.T) {
	base, _ := startTestServer(t)
	resp, err := http.Head(base + "/index.html")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) != 0 {
		t.Fatalf("HEAD returned body of %d bytes", len(body))
	}
	if resp.Header.Get("Content-Length") == "" {
		t.Fatal("HEAD missing Content-Length")
	}
}

// rawRequest sends a raw request string over TCP and returns the status code
// from the response line.
func rawRequest(t *testing.T, addr, raw string) (statusLine string, body string) {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

	if _, err := io.WriteString(conn, raw); err != nil {
		t.Fatal(err)
	}
	br := bufio.NewReader(conn)
	statusLine, err = br.ReadString('\n')
	if err != nil {
		t.Fatalf("reading status line: %v", err)
	}
	rest, _ := io.ReadAll(br)
	return strings.TrimSpace(statusLine), string(rest)
}

func TestIntegration_TraversalBlocked(t *testing.T) {
	_, addr := startTestServer(t)
	// Raw request so the path is not normalised by an HTTP client.
	status, _ := rawRequest(t, addr, "GET /../../../../etc/passwd HTTP/1.1\r\nHost: x\r\n\r\n")
	if !strings.Contains(status, "403") {
		t.Fatalf("traversal not blocked, status line = %q", status)
	}
}

func TestIntegration_EncodedTraversalBlocked(t *testing.T) {
	_, addr := startTestServer(t)
	status, _ := rawRequest(t, addr, "GET /%2e%2e/%2e%2e/etc/passwd HTTP/1.1\r\nHost: x\r\n\r\n")
	if !strings.Contains(status, "403") {
		t.Fatalf("encoded traversal not blocked, status line = %q", status)
	}
}

func TestIntegration_MalformedRequest(t *testing.T) {
	_, addr := startTestServer(t)
	status, _ := rawRequest(t, addr, "GET /\r\n\r\n")
	if !strings.Contains(status, "400") {
		t.Fatalf("malformed request status line = %q", status)
	}
}

func TestIntegration_Concurrent(t *testing.T) {
	base, _ := startTestServer(t)
	const n = 50
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func() {
			resp, err := http.Get(base + "/index.html")
			if err != nil {
				errs <- err
				return
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != 200 {
				errs <- io.ErrUnexpectedEOF
				return
			}
			errs <- nil
		}()
	}
	for i := 0; i < n; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent request %d failed: %v", i, err)
		}
	}
}

func TestNew_RejectsMissingRoot(t *testing.T) {
	if _, err := New("127.0.0.1:0", filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Fatal("expected error for missing doc root")
	}
}

func TestNew_RejectsFileRoot(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := New("127.0.0.1:0", f); err == nil {
		t.Fatal("expected error when doc root is a file")
	}
}
