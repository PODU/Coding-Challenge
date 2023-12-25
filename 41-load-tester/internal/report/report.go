// Package report renders a stats.Summary to an io.Writer as either an aligned
// human-readable table or machine-readable JSON.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"time"

	"loadtester/internal/stats"
)

// Table writes a human-readable summary.
func Table(w io.Writer, s stats.Summary) error {
	bw := &errWriter{w: w}

	bw.printf("Summary:\n")
	bw.printf("  Total requests:  %6d\n", s.Total)
	bw.printf("  Success (2XX):   %6d\n", s.Success)
	bw.printf("  Failed:          %6d\n", s.Failed)
	if s.Timeouts > 0 {
		bw.printf("    timeouts:      %6d\n", s.Timeouts)
	}
	if s.Errors > 0 {
		bw.printf("    errors:        %6d\n", s.Errors)
	}
	bw.printf("  Wall clock:      %9s\n", roundDur(s.WallClock))
	bw.printf("  Throughput:      %8.1f req/s\n", s.Throughput)

	bw.printf("\nStatus codes:\n")
	for _, kv := range sortedCounts(s.StatusCodes) {
		bw.printf("  %-8s %d\n", kv.key+":", kv.val)
	}

	bw.printf("\nLatencies (ms):          %8s %8s %8s %8s %8s %8s %8s\n",
		"min", "mean", "p50", "p90", "p95", "p99", "max")
	writeLatencyRow(bw, "Total", s.TotalStats)
	writeLatencyRow(bw, "Time to first byte", s.TTFBStats)
	writeLatencyRow(bw, "Time to last byte", s.TTLBStats)

	return bw.err
}

func writeLatencyRow(bw *errWriter, label string, l stats.LatencyStats) {
	if l.Count == 0 {
		bw.printf("  %-20s %8s %8s %8s %8s %8s %8s %8s\n",
			label+":", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a", "n/a")
		return
	}
	bw.printf("  %-20s %8s %8s %8s %8s %8s %8s %8s\n",
		label+":",
		ms(l.Min), ms(l.Mean), ms(l.P50), ms(l.P90), ms(l.P95), ms(l.P99), ms(l.Max))
}

func ms(d time.Duration) string {
	return strconv.FormatFloat(float64(d)/float64(time.Millisecond), 'f', 2, 64)
}

func roundDur(d time.Duration) time.Duration {
	switch {
	case d >= time.Second:
		return d.Round(10 * time.Millisecond)
	case d >= time.Millisecond:
		return d.Round(100 * time.Microsecond)
	default:
		return d.Round(time.Microsecond)
	}
}

type jsonLatency struct {
	Count int     `json:"count"`
	Min   float64 `json:"min_ms"`
	Mean  float64 `json:"mean_ms"`
	P50   float64 `json:"p50_ms"`
	P90   float64 `json:"p90_ms"`
	P95   float64 `json:"p95_ms"`
	P99   float64 `json:"p99_ms"`
	Max   float64 `json:"max_ms"`
}

func toJSONLatency(l stats.LatencyStats) jsonLatency {
	f := func(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
	return jsonLatency{
		Count: l.Count,
		Min:   f(l.Min), Mean: f(l.Mean), P50: f(l.P50),
		P90: f(l.P90), P95: f(l.P95), P99: f(l.P99), Max: f(l.Max),
	}
}

type jsonSummary struct {
	Total         int            `json:"total"`
	Success       int            `json:"success"`
	Failed        int            `json:"failed"`
	Timeouts      int            `json:"timeouts"`
	Errors        int            `json:"errors"`
	WallClockMS   float64        `json:"wall_clock_ms"`
	ThroughputRPS float64        `json:"throughput_rps"`
	StatusCodes   map[string]int `json:"status_codes"`
	TotalLatency  jsonLatency    `json:"total_latency"`
	TTFBLatency   jsonLatency    `json:"ttfb_latency"`
	TTLBLatency   jsonLatency    `json:"ttlb_latency"`
}

// JSON writes the summary as indented JSON.
func JSON(w io.Writer, s stats.Summary) error {
	out := jsonSummary{
		Total:         s.Total,
		Success:       s.Success,
		Failed:        s.Failed,
		Timeouts:      s.Timeouts,
		Errors:        s.Errors,
		WallClockMS:   float64(s.WallClock) / float64(time.Millisecond),
		ThroughputRPS: s.Throughput,
		StatusCodes:   s.StatusCodes,
		TotalLatency:  toJSONLatency(s.TotalStats),
		TTFBLatency:   toJSONLatency(s.TTFBStats),
		TTLBLatency:   toJSONLatency(s.TTLBStats),
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

type kv struct {
	key string
	val int
}

// sortedCounts orders numeric status codes first, then "error" and "timeout".
func sortedCounts(m map[string]int) []kv {
	out := make([]kv, 0, len(m))
	for k, v := range m {
		out = append(out, kv{k, v})
	}
	sort.Slice(out, func(i, j int) bool {
		ni, ei := strconv.Atoi(out[i].key)
		nj, ej := strconv.Atoi(out[j].key)
		switch {
		case ei == nil && ej == nil:
			return ni < nj
		case ei == nil:
			return true
		case ej == nil:
			return false
		default:
			return out[i].key < out[j].key
		}
	})
	return out
}

// errWriter remembers the first write error so printf calls can be chained.
type errWriter struct {
	w   io.Writer
	err error
}

func (e *errWriter) printf(format string, args ...any) {
	if e.err != nil {
		return
	}
	_, e.err = fmt.Fprintf(e.w, format, args...)
}
