// Package aggregate computes uptime percentage, bucketed history and latency
// percentiles from a slice of CheckResults.
package aggregate

import (
	"math"
	"sort"
	"time"

	"github.com/example/uptime/internal/monitor"
)

// Uptime summarises availability over a set of results.
type Uptime struct {
	Total   int     `json:"total"`   // up + down checks considered
	Up      int     `json:"up"`      // checks that were StatusUp
	Down    int     `json:"down"`    // checks that were StatusDown
	Percent float64 `json:"percent"` // 0..100; NaN when Total == 0
}

// UptimePercent computes availability over results in [from, to], ignoring
// StatusUnknown. A zero to means no upper bound. With no checks in the window
// Percent is NaN, which callers show as "no data".
func UptimePercent(results []monitor.CheckResult, from, to time.Time) Uptime {
	var u Uptime
	for _, r := range results {
		if r.Timestamp.Before(from) {
			continue
		}
		if !to.IsZero() && r.Timestamp.After(to) {
			continue
		}
		switch r.Status {
		case monitor.StatusUp:
			u.Up++
		case monitor.StatusDown:
			u.Down++
		}
	}
	u.Total = u.Up + u.Down
	if u.Total == 0 {
		u.Percent = math.NaN()
		return u
	}
	u.Percent = float64(u.Up) / float64(u.Total) * 100
	return u
}

// Bucket is one point on the history graphs: an availability ratio plus latency
// stats for the round-trip-time chart.
type Bucket struct {
	Start      time.Time `json:"start"`
	Total      int       `json:"total"`
	Up         int       `json:"up"`
	UpRatio    float64   `json:"up_ratio"`
	AvgLatency float64   `json:"avg_latency_ms"`
	P50Latency float64   `json:"p50_latency_ms"`
	P95Latency float64   `json:"p95_latency_ms"`
}

// History groups results in [from, to] into buckets of the given width for the
// dashboard graphs. Empty buckets are kept so the time axis has no gaps.
func History(results []monitor.CheckResult, from, to time.Time, bucket time.Duration) []Bucket {
	if bucket <= 0 || !to.After(from) {
		return nil
	}
	n := int(to.Sub(from)/bucket) + 1
	buckets := make([]Bucket, n)
	lat := make([][]float64, n)
	for i := range buckets {
		buckets[i].Start = from.Add(time.Duration(i) * bucket)
	}
	for _, r := range results {
		if r.Timestamp.Before(from) || r.Timestamp.After(to) {
			continue
		}
		idx := int(r.Timestamp.Sub(from) / bucket)
		if idx < 0 || idx >= n {
			continue
		}
		b := &buckets[idx]
		switch r.Status {
		case monitor.StatusUp:
			b.Up++
			b.Total++
		case monitor.StatusDown:
			b.Total++
		default:
			continue
		}
		if r.Latency > 0 {
			lat[idx] = append(lat[idx], float64(r.Latency.Microseconds())/1000.0)
		}
	}
	for i := range buckets {
		b := &buckets[i]
		if b.Total > 0 {
			b.UpRatio = float64(b.Up) / float64(b.Total)
		}
		if len(lat[i]) > 0 {
			b.AvgLatency = mean(lat[i])
			b.P50Latency = percentile(lat[i], 50)
			b.P95Latency = percentile(lat[i], 95)
		}
	}
	return buckets
}

// LatencyPercentiles returns p50/p95/p99 round-trip latencies over results that
// recorded a non-zero latency. Missing data yields zero durations.
func LatencyPercentiles(results []monitor.CheckResult) (p50, p95, p99 time.Duration) {
	vals := make([]float64, 0, len(results))
	for _, r := range results {
		if r.Latency > 0 {
			vals = append(vals, float64(r.Latency.Nanoseconds()))
		}
	}
	if len(vals) == 0 {
		return 0, 0, 0
	}
	return time.Duration(percentile(vals, 50)),
		time.Duration(percentile(vals, 95)),
		time.Duration(percentile(vals, 99))
}

func mean(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	var sum float64
	for _, x := range v {
		sum += x
	}
	return sum / float64(len(v))
}

// percentile returns the p-th percentile (0..100) using the nearest-rank method.
func percentile(v []float64, p float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := make([]float64, len(v))
	copy(s, v)
	sort.Float64s(s)
	if p <= 0 {
		return s[0]
	}
	if p >= 100 {
		return s[len(s)-1]
	}
	rank := int(math.Ceil(p/100*float64(len(s)))) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= len(s) {
		rank = len(s) - 1
	}
	return s[rank]
}
