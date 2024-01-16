//go:build live

// Live integration test. Excluded from the default offline build; run with:
//
//	go test -tags=live ./internal/server/ -run TestLiveUpstream -v
//
// It starts the forwarder against the real 8.8.8.8 resolver and resolves a real
// name, requiring outbound UDP/53. Skipped unless LIVE_DNS=1 is also set.
package server

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"dns-forwarder/internal/cache"
	"dns-forwarder/internal/dnsmsg"
	"dns-forwarder/internal/upstream"
)

func TestLiveUpstream(t *testing.T) {
	if os.Getenv("LIVE_DNS") != "1" {
		t.Skip("set LIVE_DNS=1 to run the live upstream test")
	}
	up := upstream.New("8.8.8.8:53", 5*time.Second)
	s := New(Config{Listen: "127.0.0.1:0", Timeout: 5 * time.Second}, up, cache.New(), nil)
	if err := s.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	}()

	conn, err := net.DialUDP("udp", nil, s.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	q := query(0x7777, "www.google.com", dnsmsg.TypeA)
	if _, err := conn.Write(q); err != nil {
		t.Fatalf("write: %v", err)
	}
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 65535)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	m, err := dnsmsg.Decode(buf[:n])
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if m.Header.RCode != dnsmsg.RCodeNoError {
		t.Fatalf("RCode = %d, want NOERROR", m.Header.RCode)
	}
	hasA := false
	for _, a := range m.Answers {
		if a.Type == dnsmsg.TypeA {
			hasA = true
		}
	}
	if !hasA {
		t.Fatalf("no A record in answer (answers=%d)", len(m.Answers))
	}
	t.Logf("resolved www.google.com -> %d answer record(s)", len(m.Answers))
}
