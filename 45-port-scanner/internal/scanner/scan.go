package scanner

import (
	"context"
	"errors"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"
)

type State string

const (
	// Open means the TCP handshake completed: a service is listening.
	Open State = "open"
	// Closed means the host actively refused the connection (RST).
	Closed State = "closed"
	// Filtered means no answer at all: a firewall dropped the packets or the
	// host is down.
	Filtered State = "filtered"
)

type Job struct {
	Host string
	Port int
}

type Result struct {
	Host    string `json:"host"`
	Port    int    `json:"port"`
	State   State  `json:"state"`
	Service string `json:"service,omitempty"`
}

func scanPort(ctx context.Context, host string, port int, timeout time.Duration) State {
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err == nil {
		_ = conn.Close()
		return Open
	}
	return classifyDialError(err)
}

func classifyDialError(err error) State {
	if isConnRefused(err) {
		return Closed
	}
	return Filtered
}

// Scan connects to every host/port pair using concurrency workers and returns
// the results sorted by host, then port. If ctx is cancelled it returns what it
// has so far.
func Scan(ctx context.Context, hosts []string, ports []int, concurrency int, timeout time.Duration) []Result {
	if concurrency < 1 {
		concurrency = 1
	}

	jobs := make(chan Job, concurrency)
	results := make(chan Result, concurrency)

	go func() {
		defer close(jobs)
		for _, h := range hosts {
			for _, p := range ports {
				select {
				case <-ctx.Done():
					return
				case jobs <- Job{Host: h, Port: p}:
				}
			}
		}
	}()

	var wg sync.WaitGroup
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			for job := range jobs {
				if ctx.Err() != nil {
					return
				}
				state := scanPort(ctx, job.Host, job.Port, timeout)
				select {
				case <-ctx.Done():
					return
				case results <- Result{
					Host:    job.Host,
					Port:    job.Port,
					State:   state,
					Service: ServiceName(job.Port),
				}:
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var collected []Result
	for r := range results {
		collected = append(collected, r)
	}

	sortResults(collected)
	return collected
}

func sortResults(rs []Result) {
	sort.SliceStable(rs, func(i, j int) bool {
		if rs[i].Host != rs[j].Host {
			return rs[i].Host < rs[j].Host
		}
		return rs[i].Port < rs[j].Port
	})
}

func isConnRefused(err error) bool {
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Timeout() {
		return false
	}
	for _, e := range connRefusedErrnos {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}
