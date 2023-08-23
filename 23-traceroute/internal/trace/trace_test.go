package trace

import (
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

// scriptedProber returns canned replies per probe call, in order.
type scriptedProber struct {
	replies []Reply
	i       int
}

func (s *scriptedProber) Probe(ttl, seq int) (Reply, error) {
	if s.i >= len(s.replies) {
		return Reply{TimedOut: true}, nil
	}
	r := s.replies[s.i]
	s.i++
	return r, nil
}

func (s *scriptedProber) Close() error { return nil }

func baseConfig() Config {
	return Config{
		Dest:       net.ParseIP("93.184.216.34"),
		DestName:   "example.com",
		MaxHops:    5,
		Queries:    3,
		Timeout:    time.Second,
		BasePort:   DefaultBasePort,
		PacketSize: 32,
	}
}

func TestTracer_Run_ReachesDestination(t *testing.T) {
	gw := net.ParseIP("192.168.1.1")
	dst := net.ParseIP("93.184.216.34")

	prober := &scriptedProber{replies: []Reply{
		// hop 1: gateway answers all three queries
		{From: gw, RTT: time.Millisecond},
		{From: gw, RTT: time.Millisecond},
		{From: gw, RTT: time.Millisecond},
		// hop 2: timeouts
		{TimedOut: true}, {TimedOut: true}, {TimedOut: true},
		// hop 3: destination reached
		{From: dst, RTT: 2 * time.Millisecond, DestReached: true},
		{From: dst, RTT: 2 * time.Millisecond, DestReached: true},
		{From: dst, RTT: 2 * time.Millisecond, DestReached: true},
	}}

	tr := NewTracer(baseConfig(), prober, numericResolve)
	var sb strings.Builder
	if err := tr.Run(&sb); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	out := sb.String()
	if !strings.HasPrefix(out, "traceroute to example.com (93.184.216.34), 5 hops max, 32 byte packets\n") {
		t.Errorf("unexpected header line:\n%s", out)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	// header + 3 hop lines (must stop at destination, not run to maxhops)
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4 (header + 3 hops):\n%s", len(lines), out)
	}
	if !strings.Contains(lines[2], "* * *") {
		t.Errorf("hop 2 should be a full timeout, got %q", lines[2])
	}
	if !strings.Contains(lines[3], "93.184.216.34") {
		t.Errorf("final hop should be the destination, got %q", lines[3])
	}
}

func TestTracer_Run_NotReached(t *testing.T) {
	// Prober always times out -> destination never reached.
	prober := &scriptedProber{}
	cfg := baseConfig()
	cfg.MaxHops = 3
	tr := NewTracer(cfg, prober, numericResolve)

	var sb strings.Builder
	err := tr.Run(&sb)
	if !errors.Is(err, ErrNotReached) {
		t.Fatalf("Run error = %v, want ErrNotReached", err)
	}
	lines := strings.Split(strings.TrimRight(sb.String(), "\n"), "\n")
	if len(lines) != 4 { // header + 3 timeout hops
		t.Errorf("got %d lines, want 4:\n%s", len(lines), sb.String())
	}
}

func TestModeString(t *testing.T) {
	if ModeUDP.String() != "udp" || ModeICMP.String() != "icmp" {
		t.Errorf("Mode.String mismatch: udp=%q icmp=%q", ModeUDP, ModeICMP)
	}
}
