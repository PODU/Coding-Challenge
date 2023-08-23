// Package trace implements traceroute: building probes, parsing ICMP replies,
// the TTL loop, reverse DNS and the hop table output.
//
// Only receiver.go touches raw sockets; everything else can be tested without
// elevated privileges.
package trace

import (
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// IPv4 protocol numbers.
const (
	ProtocolICMP = 1
	ProtocolUDP  = 17
)

const (
	DefaultMaxHops    = 64
	DefaultQueries    = 3
	DefaultTimeout    = 3 * time.Second
	DefaultBasePort   = 33434
	DefaultPacketSize = 32
)

// Mode selects the probe transport.
type Mode int

const (
	// ModeUDP sends UDP datagrams to high ports, like Unix traceroute.
	ModeUDP Mode = iota
	// ModeICMP sends ICMP Echo Requests, like Windows tracert.
	ModeICMP
)

func (m Mode) String() string {
	if m == ModeICMP {
		return "icmp"
	}
	return "udp"
}

// ErrNotReached is returned by Run when the trace stops at the max hop count
// without the destination answering.
var ErrNotReached = errors.New("destination not reached within max hops")

// Config describes one trace run.
type Config struct {
	Dest       net.IP
	DestName   string
	MaxHops    int
	Queries    int
	Timeout    time.Duration
	BasePort   int
	PacketSize int
	Numeric    bool
	Mode       Mode
}

// Query is the outcome of a single probe.
type Query struct {
	Addr     net.IP // nil when timed out
	RTT      time.Duration
	TimedOut bool
}

// Hop groups the queries sent at one TTL value.
type Hop struct {
	TTL     int
	Queries []Query
}

// Reply is what Prober.Probe got back.
type Reply struct {
	From        net.IP
	RTT         time.Duration
	TimedOut    bool
	DestReached bool
}

// Prober sends one probe at a given TTL and waits for the matching reply. Tests
// substitute a fake so the TTL loop runs without raw sockets.
type Prober interface {
	Probe(ttl, seq int) (Reply, error)
	Close() error
}

// Tracer runs the TTL loop and prints the hop table.
type Tracer struct {
	cfg     Config
	prober  Prober
	resolve func(net.IP) string
}

// NewTracer returns a Tracer that names hops with resolve.
func NewTracer(cfg Config, p Prober, resolve func(net.IP) string) *Tracer {
	return &Tracer{cfg: cfg, prober: p, resolve: resolve}
}

// Run writes the header and one line per hop to w. It returns ErrNotReached if
// the destination never answers.
func (t *Tracer) Run(w io.Writer) error {
	fmt.Fprintf(w, "traceroute to %s (%s), %d hops max, %d byte packets\n",
		t.cfg.DestName, t.cfg.Dest, t.cfg.MaxHops, t.cfg.PacketSize)

	seq := 0
	reached := false
	for ttl := 1; ttl <= t.cfg.MaxHops && !reached; ttl++ {
		hop := Hop{TTL: ttl}
		for q := 0; q < t.cfg.Queries; q++ {
			rep, err := t.prober.Probe(ttl, seq)
			seq++
			if err != nil {
				return err
			}
			hop.Queries = append(hop.Queries, Query{
				Addr:     rep.From,
				RTT:      rep.RTT,
				TimedOut: rep.TimedOut,
			})
			if rep.DestReached {
				reached = true
			}
		}
		fmt.Fprintln(w, FormatHop(hop, t.resolve))
	}

	if !reached {
		return ErrNotReached
	}
	return nil
}
