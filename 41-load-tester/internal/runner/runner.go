// Package runner sends requests from a fixed-size worker pool and collects the
// samples. HTTP is behind the Requester interface so tests can fake it.
package runner

import (
	"context"
	"io"
	"net/http"
	"net/http/httptrace"
	"strings"
	"sync"
	"time"

	"loadtester/internal/stats"
)

type Job struct {
	Seq int
	URL string
}

// Requester performs one request and returns a timed sample. Implementations
// must not panic on transport errors; they encode failures in the Sample.
type Requester interface {
	Do(ctx context.Context, job Job) stats.Sample
}

// Provider yields the ordered set of targets to request.
type Provider interface {
	N() int
	At(i int) string
}

// Run makes p.N() requests using c workers and returns the samples and the
// total wall-clock time. If ctx is cancelled no new requests start, in-flight
// ones finish, and the partial results are returned.
func Run(ctx context.Context, p Provider, c int, req Requester) (*stats.Collector, time.Duration) {
	if c < 1 {
		c = 1
	}
	jobs := make(chan Job, c)
	results := make(chan stats.Sample, c)

	start := time.Now()

	go func() {
		defer close(jobs)
		for i := 0; i < p.N(); i++ {
			select {
			case jobs <- Job{Seq: i, URL: p.At(i)}:
			case <-ctx.Done():
				return
			}
		}
	}()

	var wg sync.WaitGroup
	wg.Add(c)
	for w := 0; w < c; w++ {
		go func() {
			defer wg.Done()
			for job := range jobs {
				results <- req.Do(ctx, job)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	collector := stats.NewCollector()
	for s := range results {
		collector.Add(s)
	}
	return collector, time.Since(start)
}

// HTTPRequester is the real Requester. Its client is shared by all workers so
// connections get reused.
type HTTPRequester struct {
	Client  *http.Client
	Method  string
	Headers http.Header
	Body    string
	Timeout time.Duration
}

// NewHTTPRequester sizes the connection pool for the given concurrency.
func NewHTTPRequester(method string, headers http.Header, body string, timeout time.Duration, concurrency int) *HTTPRequester {
	transport := &http.Transport{
		MaxIdleConns:        concurrency * 2,
		MaxIdleConnsPerHost: concurrency * 2,
		IdleConnTimeout:     90 * time.Second,
	}
	return &HTTPRequester{
		// No Client.Timeout: the deadline is set per request via the context
		// so timeouts can be told apart from other transport errors.
		Client:  &http.Client{Transport: transport},
		Method:  method,
		Headers: headers,
		Body:    body,
		Timeout: timeout,
	}
}

// Do sends one request and times it.
func (r *HTTPRequester) Do(ctx context.Context, job Job) stats.Sample {
	sample := stats.Sample{Seq: job.Seq}

	reqCtx := ctx
	var cancel context.CancelFunc
	if r.Timeout > 0 {
		reqCtx, cancel = context.WithTimeout(ctx, r.Timeout)
		defer cancel()
	}

	var bodyReader io.Reader
	if r.Body != "" {
		bodyReader = strings.NewReader(r.Body)
	}

	httpReq, err := http.NewRequestWithContext(reqCtx, r.Method, job.URL, bodyReader)
	if err != nil {
		sample.Err = err
		return sample
	}
	for k, vs := range r.Headers {
		for _, v := range vs {
			httpReq.Header.Add(k, v)
		}
	}

	var ttfb time.Duration
	start := time.Now()
	trace := &httptrace.ClientTrace{
		GotFirstResponseByte: func() {
			ttfb = time.Since(start)
		},
	}
	httpReq = httpReq.WithContext(httptrace.WithClientTrace(reqCtx, trace))

	resp, err := r.Client.Do(httpReq)
	if err != nil {
		sample.Err = err
		sample.Timeout = isTimeout(reqCtx, err)
		return sample
	}
	defer resp.Body.Close()

	// Read the whole body: TTLB depends on it, and so does connection reuse.
	n, _ := io.Copy(io.Discard, resp.Body)
	total := time.Since(start)

	sample.StatusCode = resp.StatusCode
	sample.Bytes = n
	sample.TTFB = ttfb
	if ttfb == 0 {
		// The first-byte hook did not fire; fall back to TTLB.
		sample.TTFB = total
	}
	sample.TTLB = total
	sample.Total = total
	return sample
}

func isTimeout(ctx context.Context, err error) bool {
	if ctx.Err() == context.DeadlineExceeded {
		return true
	}
	type timeouter interface{ Timeout() bool }
	if t, ok := err.(timeouter); ok {
		return t.Timeout()
	}
	return false
}
