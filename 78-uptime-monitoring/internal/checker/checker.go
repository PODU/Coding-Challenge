// Package checker probes monitors over HTTP or TCP and reports a
// monitor.CheckResult.
package checker

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"strings"
	"time"

	"github.com/example/uptime/internal/clock"
	"github.com/example/uptime/internal/monitor"
)

// Checker probes a single monitor. Failures come back as StatusDown with Err
// set, never as a Go error.
type Checker interface {
	Check(ctx context.Context, m monitor.Monitor) monitor.CheckResult
}

// Multi dispatches to the right checker based on the monitor Kind.
type Multi struct {
	HTTP Checker
	TCP  Checker
}

// NewMulti builds a dispatcher with default HTTP and TCP checkers sharing clk.
func NewMulti(clk clock.Clock) *Multi {
	return &Multi{HTTP: NewHTTP(clk), TCP: NewTCP(clk)}
}

// Check routes to the HTTP or TCP checker.
func (m *Multi) Check(ctx context.Context, mon monitor.Monitor) monitor.CheckResult {
	if mon.Kind == monitor.KindTCP {
		return m.TCP.Check(ctx, mon)
	}
	return m.HTTP.Check(ctx, mon)
}

// HTTPChecker probes HTTP(S) endpoints with HEAD (default) or GET (captures
// TTFB). Latency is measured with a monotonic stopwatch; Timestamp comes from
// the injected clock.
type HTTPChecker struct {
	clk    clock.Clock
	client *http.Client
}

// NewHTTP returns an HTTPChecker with a sane shared transport.
func NewHTTP(clk clock.Clock) *HTTPChecker {
	tr := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   4,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
	}
	return &HTTPChecker{
		clk: clk,
		client: &http.Client{
			Transport: tr,
			// No client timeout: the per-check deadline is on the context.
		},
	}
}

func (c *HTTPChecker) Check(ctx context.Context, m monitor.Monitor) monitor.CheckResult {
	res := monitor.CheckResult{
		MonitorID: m.ID,
		Timestamp: c.clk.Now(),
		Status:    monitor.StatusDown,
	}

	method := strings.ToUpper(strings.TrimSpace(m.Method))
	if method == "" {
		method = http.MethodHead
	}

	req, err := http.NewRequestWithContext(ctx, method, m.Target, nil)
	if err != nil {
		res.Err = "bad request: " + err.Error()
		return res
	}
	req.Header.Set("User-Agent", "uptime-monitor/1.0")

	var firstByte time.Duration
	start := time.Now()
	if method == http.MethodGet {
		trace := &httptrace.ClientTrace{
			GotFirstResponseByte: func() { firstByte = time.Since(start) },
		}
		req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	}

	resp, err := c.client.Do(req)
	if err != nil {
		res.Latency = time.Since(start)
		res.Err = classifyErr(ctx, err)
		return res
	}
	defer resp.Body.Close()

	// Read the whole GET body so latency covers the full response and the
	// connection can be reused.
	if method == http.MethodGet {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 8<<20))
	}
	res.Latency = time.Since(start)
	res.TTFB = firstByte
	res.StatusCode = resp.StatusCode

	expect := m.ExpectStatus
	if len(expect) == 0 {
		expect = []monitor.StatusRange{{Lo: 200, Hi: 399}}
	}
	switch {
	case !monitor.StatusMatches(expect, resp.StatusCode):
		res.Status = monitor.StatusDown
		res.Err = "unexpected status " + resp.Status
	case m.LatencyThreshold > 0 && res.Latency > m.LatencyThreshold:
		res.Status = monitor.StatusDown
		res.Err = "latency over threshold"
	default:
		res.Status = monitor.StatusUp
	}
	return res
}

func classifyErr(ctx context.Context, err error) string {
	if ctx.Err() == context.DeadlineExceeded {
		return "timeout"
	}
	if ctx.Err() == context.Canceled {
		return "canceled"
	}
	var nerr net.Error
	if errors.As(err, &nerr) && nerr.Timeout() {
		return "timeout"
	}
	return err.Error()
}

// TCPChecker probes a raw host:port by dialing a TCP connection.
type TCPChecker struct {
	clk    clock.Clock
	dialer *net.Dialer
}

// NewTCP returns a TCPChecker.
func NewTCP(clk clock.Clock) *TCPChecker {
	return &TCPChecker{clk: clk, dialer: &net.Dialer{}}
}

func (c *TCPChecker) Check(ctx context.Context, m monitor.Monitor) monitor.CheckResult {
	res := monitor.CheckResult{
		MonitorID: m.ID,
		Timestamp: c.clk.Now(),
		Status:    monitor.StatusDown,
	}
	start := time.Now()
	conn, err := c.dialer.DialContext(ctx, "tcp", m.Target)
	res.Latency = time.Since(start)
	if err != nil {
		res.Err = classifyErr(ctx, err)
		return res
	}
	_ = conn.Close()
	if m.LatencyThreshold > 0 && res.Latency > m.LatencyThreshold {
		res.Err = "latency over threshold"
		return res
	}
	res.Status = monitor.StatusUp
	return res
}
