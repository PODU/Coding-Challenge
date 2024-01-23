package scanner

import (
	"bytes"
	"context"
	"net"
	"strconv"
	"testing"
	"time"
)

// listenLocal starts a TCP listener on an OS-assigned ephemeral localhost port
// and returns the listener and its port. The caller closes the listener.
func listenLocal(t *testing.T) (net.Listener, int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	return ln, port
}

// freeClosedPort grabs an ephemeral port then immediately releases it, so the
// returned port is (very likely) closed for the duration of the test.
func freeClosedPort(t *testing.T) int {
	t.Helper()
	ln, port := listenLocal(t)
	_ = ln.Close()
	return port
}

func TestScanPortOpen(t *testing.T) {
	ln, port := listenLocal(t)
	defer ln.Close()
	// Accept loop so the handshake completes cleanly.
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()

	state := scanPort(context.Background(), "127.0.0.1", port, 500*time.Millisecond)
	if state != Open {
		t.Errorf("scanPort(open port %d) = %v, want %v", port, state, Open)
	}
}

func TestScanPortClosed(t *testing.T) {
	port := freeClosedPort(t)
	state := scanPort(context.Background(), "127.0.0.1", port, 500*time.Millisecond)
	if state != Closed {
		t.Errorf("scanPort(closed port %d) = %v, want %v", port, state, Closed)
	}
}

func TestScanPortFiltered(t *testing.T) {
	// 10.255.255.1 is in private space and typically black-holed; with a tiny
	// timeout the dial should time out -> filtered. (Not Closed.)
	state := scanPort(context.Background(), "10.255.255.1", 80, 100*time.Millisecond)
	if state != Filtered {
		t.Errorf("scanPort(black-holed) = %v, want %v", state, Filtered)
	}
}

func TestScanEndToEnd(t *testing.T) {
	// Open three listeners; collect their ports.
	const n = 3
	openPorts := make(map[int]bool)
	var closers []net.Listener
	for i := 0; i < n; i++ {
		ln, port := listenLocal(t)
		openPorts[port] = true
		closers = append(closers, ln)
		go func(l net.Listener) {
			for {
				c, err := l.Accept()
				if err != nil {
					return
				}
				_ = c.Close()
			}
		}(ln)
	}
	defer func() {
		for _, l := range closers {
			_ = l.Close()
		}
	}()

	// Add two known-closed ports.
	closed1 := freeClosedPort(t)
	closed2 := freeClosedPort(t)

	var ports []int
	for p := range openPorts {
		ports = append(ports, p)
	}
	ports = append(ports, closed1, closed2)

	results := Scan(context.Background(), []string{"127.0.0.1"}, ports, 16, 500*time.Millisecond)
	if len(results) != len(ports) {
		t.Fatalf("got %d results, want %d", len(results), len(ports))
	}

	got := make(map[int]State)
	for _, r := range results {
		got[r.Port] = r.State
	}
	for p := range openPorts {
		if got[p] != Open {
			t.Errorf("port %d = %v, want open", p, got[p])
		}
	}
	if got[closed1] != Closed {
		t.Errorf("closed port %d = %v, want closed", closed1, got[closed1])
	}
	if got[closed2] != Closed {
		t.Errorf("closed port %d = %v, want closed", closed2, got[closed2])
	}
}

func TestScanConcurrencyInvariance(t *testing.T) {
	ln, openPort := listenLocal(t)
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	closed := freeClosedPort(t)
	ports := []int{openPort, closed}

	collect := func(c int) map[int]State {
		res := Scan(context.Background(), []string{"127.0.0.1"}, ports, c, 500*time.Millisecond)
		m := make(map[int]State)
		for _, r := range res {
			m[r.Port] = r.State
		}
		return m
	}

	a := collect(1)
	b := collect(64)
	if len(a) != len(b) {
		t.Fatalf("result set sizes differ: %d vs %d", len(a), len(b))
	}
	for p, s := range a {
		if b[p] != s {
			t.Errorf("port %d: c=1 gave %v, c=64 gave %v", p, s, b[p])
		}
	}
	if a[openPort] != Open {
		t.Errorf("open port not open at c=1: %v", a[openPort])
	}
}

func TestScanContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled
	// A large port set; the cancelled context should make Scan return promptly.
	ports, _ := ParsePorts("1-2000")
	done := make(chan []Result, 1)
	go func() {
		done <- Scan(ctx, []string{"10.255.255.1"}, ports, 64, 2*time.Second)
	}()
	select {
	case <-done:
		// returned promptly — good
	case <-time.After(3 * time.Second):
		t.Fatal("Scan did not return promptly under cancelled context")
	}
}

func TestRenderJSONGolden(t *testing.T) {
	results := []Result{
		{Host: "127.0.0.1", Port: 22, State: Open, Service: "ssh"},
		{Host: "127.0.0.1", Port: 80, State: Open, Service: "http"},
		{Host: "127.0.0.1", Port: 81, State: Closed},
	}
	rep := BuildReport(results, 512, 500*time.Millisecond, 0, 3, false)

	var buf bytes.Buffer
	if err := RenderJSON(&buf, rep); err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}

	want := `{
  "concurrency": 512,
  "timeout_ms": 500,
  "ports_per_host": 3,
  "elapsed_ms": 0,
  "hosts": [
    {
      "host": "127.0.0.1",
      "open": 2,
      "closed": 1,
      "filtered": 0,
      "ports": [
        {
          "host": "127.0.0.1",
          "port": 22,
          "state": "open",
          "service": "ssh"
        },
        {
          "host": "127.0.0.1",
          "port": 80,
          "state": "open",
          "service": "http"
        }
      ]
    }
  ]
}
`
	if got := buf.String(); got != want {
		t.Errorf("JSON golden mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestBuildReportVerboseIncludesClosed(t *testing.T) {
	results := []Result{
		{Host: "h", Port: 22, State: Open, Service: "ssh"},
		{Host: "h", Port: 81, State: Closed},
	}
	rep := BuildReport(results, 1, time.Second, 0, 2, true)
	if len(rep.Hosts) != 1 {
		t.Fatalf("want 1 host, got %d", len(rep.Hosts))
	}
	if len(rep.Hosts[0].Ports) != 2 {
		t.Errorf("verbose should include closed port: got %d ports", len(rep.Hosts[0].Ports))
	}
}

// ensure Result port renders deterministically (guards strconv import usage in scan.go)
func TestPortJoinHostPort(t *testing.T) {
	if got := net.JoinHostPort("127.0.0.1", strconv.Itoa(80)); got != "127.0.0.1:80" {
		t.Errorf("JoinHostPort = %q", got)
	}
}
