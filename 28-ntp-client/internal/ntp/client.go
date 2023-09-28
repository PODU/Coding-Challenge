package ntp

import (
	"fmt"
	"net"
	"strings"
	"time"
)

// DefaultServer is the NTP server queried when none is specified.
const DefaultServer = "pool.ntp.org:123"

// DefaultPort is appended to a server address that has no port.
const DefaultPort = "123"

// DefaultTimeout is the default UDP read timeout.
const DefaultTimeout = 5 * time.Second

// Options configures Query.
type Options struct {
	Timeout time.Duration // zero means DefaultTimeout
}

// Result is the outcome of a successful query.
type Result struct {
	Server string

	Offset time.Duration // how far the local clock is behind the server
	Delay  time.Duration // round trip, minus the server's processing time

	T0 time.Time // originate  (client transmit)
	T1 time.Time // receive    (server receive)
	T2 time.Time // transmit   (server transmit)
	T3 time.Time // destination (client receive)

	Stratum       uint8
	LeapIndicator uint8
	Version       uint8
	Precision     int8
	ReferenceID   uint32
	RefIDString   string
}

// CalculateOffset returns the clock offset theta:
//
//	theta = ((t1 - t0) + (t2 - t3)) / 2
func CalculateOffset(t0, t1, t2, t3 time.Time) time.Duration {
	return (t1.Sub(t0) + t2.Sub(t3)) / 2
}

// CalculateDelay returns the round-trip delay delta:
//
//	delta = (t3 - t0) - (t2 - t1)
func CalculateDelay(t0, t1, t2, t3 time.Time) time.Duration {
	return t3.Sub(t0) - t2.Sub(t1)
}

// normaliseServer adds the default port if server has none.
func normaliseServer(server string) string {
	if server == "" {
		return DefaultServer
	}
	if _, _, err := net.SplitHostPort(server); err != nil {
		// A bracketed IPv6 literal without a port; leave it alone.
		if strings.HasPrefix(server, "[") {
			return server
		}
		return net.JoinHostPort(server, DefaultPort)
	}
	return server
}

// Query does one SNTP exchange with server. Protocol problems are reported as
// a *KissOfDeathError or one of the Err* values; anything else is a network
// error.
func Query(server string, opts Options) (*Result, error) {
	addr := normaliseServer(server)
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("ntp: resolve %q: %w", addr, err)
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, fmt.Errorf("ntp: dial %q: %w", addr, err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, fmt.Errorf("ntp: set deadline: %w", err)
	}

	// t0 goes out as our transmit time. The server copies it into
	// OriginateTime, which is how we match the reply to this request.
	t0 := time.Now()
	transmit := TimestampFromTime(t0)
	req := NewClientRequest(transmit)

	if _, err := conn.Write(req.Marshal()); err != nil {
		return nil, fmt.Errorf("ntp: write request: %w", err)
	}

	buf := make([]byte, PacketSize)
	n, err := conn.Read(buf)
	t3 := time.Now()
	if err != nil {
		return nil, fmt.Errorf("ntp: read response: %w", err)
	}

	resp, err := Unmarshal(buf[:n])
	if err != nil {
		return nil, err
	}

	if err := resp.Validate(transmit); err != nil {
		return nil, err
	}

	t1 := resp.ReceiveTime.Time()
	t2 := resp.TransmitTime.Time()

	return &Result{
		Server:        addr,
		Offset:        CalculateOffset(t0, t1, t2, t3),
		Delay:         CalculateDelay(t0, t1, t2, t3),
		T0:            t0.UTC(),
		T1:            t1,
		T2:            t2,
		T3:            t3.UTC(),
		Stratum:       resp.Stratum,
		LeapIndicator: resp.LeapIndicator(),
		Version:       resp.Version(),
		Precision:     resp.Precision,
		ReferenceID:   resp.ReferenceID,
		RefIDString:   resp.ReferenceIDString(),
	}, nil
}

// QueryBest queries samples times and keeps the result with the lowest delay,
// since that gives the most accurate offset. It only fails if every query
// fails.
func QueryBest(server string, samples int, opts Options) (*Result, error) {
	if samples < 1 {
		samples = 1
	}
	var best *Result
	var lastErr error
	for i := 0; i < samples; i++ {
		res, err := Query(server, opts)
		if err != nil {
			lastErr = err
			// The server told us to back off, so stop now.
			if _, ok := err.(*KissOfDeathError); ok {
				return nil, err
			}
			continue
		}
		if best == nil || res.Delay < best.Delay {
			best = res
		}
	}
	if best == nil {
		return nil, lastErr
	}
	return best, nil
}
