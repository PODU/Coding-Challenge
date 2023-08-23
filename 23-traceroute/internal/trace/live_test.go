package trace

import (
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

// TestLiveTrace performs a real trace and therefore needs raw-socket
// privileges (Administrator on Windows / root on Unix) and network access. It
// is skipped unless TRACEROUTE_LIVE=1 so the default `go test` stays green
// without elevation.
func TestLiveTrace(t *testing.T) {
	if os.Getenv("TRACEROUTE_LIVE") != "1" {
		t.Skip("set TRACEROUTE_LIVE=1 (with elevation) to run the live trace test")
	}

	dst := net.ParseIP("127.0.0.1")
	cfg := Config{
		Dest:       dst,
		DestName:   "localhost",
		MaxHops:    5,
		Queries:    2,
		Timeout:    2 * time.Second,
		BasePort:   DefaultBasePort,
		PacketSize: 32,
		Mode:       ModeICMP, // most reliable on Windows
	}

	prober, err := NewProber(cfg)
	if err != nil {
		t.Fatalf("NewProber (needs elevation): %v", err)
	}
	defer prober.Close()

	resolver := NewHostResolver(DefaultResolver(), true)
	var sb strings.Builder
	if err := NewTracer(cfg, prober, resolver.Host).Run(&sb); err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := sb.String()
	t.Logf("live trace output:\n%s", out)
	if !strings.Contains(out, "127.0.0.1") {
		t.Errorf("expected destination 127.0.0.1 in output:\n%s", out)
	}
}
