package aggregate

import (
	"math"
	"testing"
	"time"

	"github.com/example/uptime/internal/monitor"
)

func mkResults(base time.Time, statuses []monitor.Status, latMs []int) []monitor.CheckResult {
	out := make([]monitor.CheckResult, len(statuses))
	for i, s := range statuses {
		var lat time.Duration
		if i < len(latMs) {
			lat = time.Duration(latMs[i]) * time.Millisecond
		}
		out[i] = monitor.CheckResult{
			MonitorID: "m",
			Timestamp: base.Add(time.Duration(i) * time.Minute),
			Status:    s,
			Latency:   lat,
		}
	}
	return out
}

func TestUptimePercent(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	up, down := monitor.StatusUp, monitor.StatusDown
	results := mkResults(base, []monitor.Status{up, up, up, up, up, up, up, up, down, down}, nil)

	u := UptimePercent(results, base, base.Add(time.Hour))
	if u.Total != 10 || u.Up != 8 || u.Down != 2 {
		t.Fatalf("counts = %+v", u)
	}
	if math.Abs(u.Percent-80.0) > 1e-9 {
		t.Fatalf("percent = %v, want 80.0", u.Percent)
	}
}

func TestUptimePercent_EmptyWindowIsNaN(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	u := UptimePercent(nil, base, base.Add(time.Hour))
	if u.Total != 0 || !math.IsNaN(u.Percent) {
		t.Fatalf("expected NaN percent and zero total, got %+v", u)
	}
}

func TestUptimePercent_IgnoresUnknownAndOutOfWindow(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	results := []monitor.CheckResult{
		{Timestamp: base.Add(-time.Hour), Status: monitor.StatusUp},       // before window
		{Timestamp: base.Add(time.Minute), Status: monitor.StatusUnknown}, // ignored
		{Timestamp: base.Add(2 * time.Minute), Status: monitor.StatusUp},
		{Timestamp: base.Add(3 * time.Minute), Status: monitor.StatusDown},
	}
	u := UptimePercent(results, base, base.Add(10*time.Minute))
	if u.Total != 2 || u.Up != 1 {
		t.Fatalf("counts = %+v", u)
	}
	if math.Abs(u.Percent-50.0) > 1e-9 {
		t.Fatalf("percent = %v, want 50.0", u.Percent)
	}
}

func TestHistoryBucketing(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	// 4 results within a 2-minute window, bucketed per minute.
	results := []monitor.CheckResult{
		{Timestamp: base.Add(10 * time.Second), Status: monitor.StatusUp, Latency: 100 * time.Millisecond},
		{Timestamp: base.Add(30 * time.Second), Status: monitor.StatusDown, Latency: 200 * time.Millisecond},
		{Timestamp: base.Add(70 * time.Second), Status: monitor.StatusUp, Latency: 50 * time.Millisecond},
		{Timestamp: base.Add(90 * time.Second), Status: monitor.StatusUp, Latency: 70 * time.Millisecond},
	}
	buckets := History(results, base, base.Add(2*time.Minute), time.Minute)
	if len(buckets) != 3 {
		t.Fatalf("expected 3 buckets, got %d", len(buckets))
	}
	if buckets[0].Total != 2 || buckets[0].Up != 1 {
		t.Fatalf("bucket0 = %+v", buckets[0])
	}
	if math.Abs(buckets[0].UpRatio-0.5) > 1e-9 {
		t.Fatalf("bucket0 ratio = %v", buckets[0].UpRatio)
	}
	if buckets[1].Total != 2 || buckets[1].Up != 2 {
		t.Fatalf("bucket1 = %+v", buckets[1])
	}
	if buckets[0].AvgLatency <= 0 {
		t.Fatal("expected latency stats in bucket0")
	}
}

func TestLatencyPercentiles(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	lat := []int{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	statuses := make([]monitor.Status, len(lat))
	for i := range statuses {
		statuses[i] = monitor.StatusUp
	}
	results := mkResults(base, statuses, lat)

	p50, p95, p99 := LatencyPercentiles(results)
	if p50 != 50*time.Millisecond {
		t.Fatalf("p50 = %s, want 50ms", p50)
	}
	if p95 != 100*time.Millisecond {
		t.Fatalf("p95 = %s, want 100ms", p95)
	}
	if p99 != 100*time.Millisecond {
		t.Fatalf("p99 = %s, want 100ms", p99)
	}
}

func TestLatencyPercentiles_Empty(t *testing.T) {
	p50, p95, p99 := LatencyPercentiles(nil)
	if p50 != 0 || p95 != 0 || p99 != 0 {
		t.Fatalf("expected zeros, got %s %s %s", p50, p95, p99)
	}
}
