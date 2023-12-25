// Package stats collects per-request samples and summarizes them.
package stats

import (
	"math"
	"sort"
	"strconv"
	"time"
)

// Sample is one recorded request outcome.
type Sample struct {
	Seq        int
	StatusCode int   // 0 if no response was received
	Err        error // transport error; nil whenever there is a response
	Timeout    bool  // the failure was the per-request timeout
	Total      time.Duration
	TTFB       time.Duration // time to first response byte
	TTLB       time.Duration // time to last response byte
	Bytes      int64
}

// timed reports whether a response arrived. Only those samples count toward
// latency statistics; errors and timeouts only show up in the counts.
func (s Sample) timed() bool {
	return s.Err == nil && s.StatusCode != 0
}

// success means a 2XX response.
func (s Sample) success() bool {
	return s.Err == nil && s.StatusCode >= 200 && s.StatusCode < 300
}

// Collector accumulates samples. It is not safe for concurrent use; the runner
// feeds it from one goroutine.
type Collector struct {
	samples []Sample
}

func NewCollector() *Collector { return &Collector{} }

func (c *Collector) Add(s Sample) { c.samples = append(c.samples, s) }

func (c *Collector) Len() int { return len(c.samples) }

// LatencyStats summarizes one latency metric. Count == 0 means there was no
// data.
type LatencyStats struct {
	Count int
	Min   time.Duration
	Max   time.Duration
	Mean  time.Duration
	P50   time.Duration
	P90   time.Duration
	P95   time.Duration
	P99   time.Duration
}

// Summary is the result of summarizing a run.
type Summary struct {
	Total      int
	Success    int // 2XX responses
	Failed     int // everything else
	Timeouts   int // part of Failed
	Errors     int // part of Failed: transport errors other than timeouts
	WallClock  time.Duration
	Throughput float64 // requests per second
	// StatusCodes counts responses by status code ("200"), plus "timeout" and
	// "error".
	StatusCodes map[string]int
	TotalStats  LatencyStats
	TTFBStats   LatencyStats
	TTLBStats   LatencyStats
}

func (c *Collector) Summarize(wallClock time.Duration) Summary {
	return Summarize(c.samples, wallClock)
}

// Summarize computes a Summary from a slice of samples.
func Summarize(samples []Sample, wallClock time.Duration) Summary {
	sum := Summary{
		Total:       len(samples),
		WallClock:   wallClock,
		StatusCodes: make(map[string]int),
	}

	var totals, ttfbs, ttlbs []time.Duration
	for _, s := range samples {
		if s.success() {
			sum.Success++
		} else {
			sum.Failed++
			if s.Timeout {
				sum.Timeouts++
			} else if s.Err != nil {
				sum.Errors++
			}
		}

		sum.StatusCodes[statusLabel(s)]++

		if s.timed() {
			totals = append(totals, s.Total)
			ttfbs = append(ttfbs, s.TTFB)
			ttlbs = append(ttlbs, s.TTLB)
		}
	}

	if secs := wallClock.Seconds(); secs > 0 {
		sum.Throughput = float64(len(samples)) / secs
	}

	sum.TotalStats = computeLatency(totals)
	sum.TTFBStats = computeLatency(ttfbs)
	sum.TTLBStats = computeLatency(ttlbs)
	return sum
}

func statusLabel(s Sample) string {
	if s.Timeout {
		return "timeout"
	}
	if s.Err != nil {
		return "error"
	}
	return strconv.Itoa(s.StatusCode)
}

// computeLatency sorts a copy of ds, leaving the caller's slice alone.
func computeLatency(ds []time.Duration) LatencyStats {
	n := len(ds)
	if n == 0 {
		return LatencyStats{}
	}
	sorted := make([]time.Duration, n)
	copy(sorted, ds)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	var total time.Duration
	for _, d := range sorted {
		total += d
	}

	return LatencyStats{
		Count: n,
		Min:   sorted[0],
		Max:   sorted[n-1],
		Mean:  time.Duration(int64(total) / int64(n)),
		P50:   percentile(sorted, 50),
		P90:   percentile(sorted, 90),
		P95:   percentile(sorted, 95),
		P99:   percentile(sorted, 99),
	}
}

// percentile returns the p-th percentile of a sorted slice by nearest rank.
func percentile(sorted []time.Duration, p float64) time.Duration {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	rank := int(math.Ceil(p/100*float64(n))) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= n {
		rank = n - 1
	}
	return sorted[rank]
}
