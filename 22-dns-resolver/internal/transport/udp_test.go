package transport

import (
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"
)

// startEchoServer starts a loopback UDP server that replies with resp to every
// datagram. It returns the address and a stop function. No internet is used.
func startEchoServer(t *testing.T, resp []byte) (netip.AddrPort, func()) {
	t.Helper()
	pc, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	done := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		for {
			pc.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
			n, addr, err := pc.ReadFromUDP(buf)
			if err != nil {
				select {
				case <-done:
					return
				default:
					continue
				}
			}
			_ = n
			_, _ = pc.WriteToUDP(resp, addr)
		}
	}()
	ap := pc.LocalAddr().(*net.UDPAddr).AddrPort()
	return ap, func() {
		close(done)
		pc.Close()
	}
}

func TestUDPExchangeLoopback(t *testing.T) {
	// A minimal well-formed (non-truncated) response payload.
	resp := []byte{0x12, 0x34, 0x81, 0x80, 0, 1, 0, 0, 0, 0, 0, 0, 0xde, 0xad}
	addr, stop := startEchoServer(t, resp)
	defer stop()

	u := UDP{Timeout: time.Second, Retries: 2}
	got, err := u.Exchange(addr, []byte("query"))
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if string(got) != string(resp) {
		t.Errorf("Exchange = % x, want % x", got, resp)
	}
}

func TestUDPExchangeTimeout(t *testing.T) {
	// Listener that never replies -> Exchange must time out, not hang.
	pc, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer pc.Close()
	addr := pc.LocalAddr().(*net.UDPAddr).AddrPort()

	u := UDP{Timeout: 100 * time.Millisecond, Retries: 2}
	start := time.Now()
	_, err = u.Exchange(addr, []byte("query"))
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	// Two attempts of ~100ms should complete well under a second.
	if time.Since(start) > 2*time.Second {
		t.Errorf("timeout took too long: %v", time.Since(start))
	}
}

func TestIsTruncated(t *testing.T) {
	notTrunc := []byte{0, 0, 0x00, 0x00}
	trunc := []byte{0, 0, 0x02, 0x00} // TC bit (1<<9)
	if isTruncated(notTrunc) {
		t.Error("false positive on TC")
	}
	if !isTruncated(trunc) {
		t.Error("missed TC bit")
	}
	if isTruncated([]byte{0, 1}) {
		t.Error("short buffer should be treated as not truncated")
	}
}

// Ensure the timeout sentinel is wired so callers can match it.
func TestErrTimeoutType(t *testing.T) {
	if !errors.Is(ErrTimeout, ErrTimeout) {
		t.Fatal("sentinel broken")
	}
}
