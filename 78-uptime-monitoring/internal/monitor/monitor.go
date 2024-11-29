// Package monitor defines monitors, their status and the CheckResult each probe
// produces.
package monitor

import (
	"fmt"
	"strings"
	"time"
)

// Status is the health of a monitor at a point in time.
type Status int

const (
	// StatusUnknown means no check has produced a verdict yet.
	StatusUnknown Status = iota
	// StatusUp means the last check succeeded.
	StatusUp
	// StatusDown means the last check failed (transport error, unexpected
	// HTTP code, or latency over the configured threshold).
	StatusDown
)

func (s Status) String() string {
	switch s {
	case StatusUp:
		return "up"
	case StatusDown:
		return "down"
	default:
		return "unknown"
	}
}

// Kind is the probe type for a monitor.
type Kind int

const (
	// KindHTTP probes an HTTP(S) URL with HEAD or GET.
	KindHTTP Kind = iota
	// KindTCP probes a raw host:port by dialing a TCP connection.
	KindTCP
)

func (k Kind) String() string {
	switch k {
	case KindTCP:
		return "tcp"
	default:
		return "http"
	}
}

// ParseKind parses "http" or "tcp" (case-insensitive).
func ParseKind(s string) (Kind, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "http", "https":
		return KindHTTP, nil
	case "tcp":
		return KindTCP, nil
	default:
		return 0, fmt.Errorf("invalid kind %q (want http|tcp)", s)
	}
}

// StatusRange is an inclusive range of acceptable HTTP status codes, e.g.
// {Lo:200, Hi:399} accepts every 2xx and 3xx response.
type StatusRange struct {
	Lo int
	Hi int
}

// Contains reports whether code falls within the range (inclusive).
func (r StatusRange) Contains(code int) bool { return code >= r.Lo && code <= r.Hi }

func (r StatusRange) String() string {
	if r.Lo == r.Hi {
		return fmt.Sprintf("%d", r.Lo)
	}
	return fmt.Sprintf("%d-%d", r.Lo, r.Hi)
}

// StatusMatches reports whether code is accepted by any of the ranges. An empty
// ranges slice matches nothing.
func StatusMatches(ranges []StatusRange, code int) bool {
	for _, r := range ranges {
		if r.Contains(code) {
			return true
		}
	}
	return false
}

// Monitor is a fully-resolved target to probe (defaults already applied).
type Monitor struct {
	ID               string
	Name             string
	Kind             Kind
	Target           string        // URL for http, host:port for tcp
	Method           string        // "HEAD" or "GET" (http only)
	Interval         time.Duration // how often to probe
	Timeout          time.Duration // per-check deadline
	LatencyThreshold time.Duration // 0 disables the latency SLO
	ExpectStatus     []StatusRange // acceptable HTTP codes (http only)
	AlertThreshold   int           // consecutive failures before a DOWN alert
	Enabled          bool
}

// CheckResult is a single time-series data point produced by a probe.
type CheckResult struct {
	MonitorID  string        `json:"monitor_id"`
	Timestamp  time.Time     `json:"timestamp"`
	Status     Status        `json:"status"`
	Latency    time.Duration `json:"latency_ms"`
	TTFB       time.Duration `json:"ttfb_ms"`
	StatusCode int           `json:"status_code"`
	Err        string        `json:"err,omitempty"`
}
