package stats

import (
	"context"
	"errors"
	"testing"
	"time"
)

func msDur(n int) time.Duration { return time.Duration(n) * time.Millisecond }

func TestPercentileNearestRank(t *testing.T) {
	// Sorted 1..100 ms.
	sorted := make([]time.Duration, 100)
	for i := 0; i < 100; i++ {
		sorted[i] = msDur(i + 1)
	}
	cases := []struct {
		p    float64
		want time.Duration
	}{
		{50, msDur(50)},
		{90, msDur(90)},
		{95, msDur(95)},
		{99, msDur(99)},
		{100, msDur(100)},
	}
	for _, c := range cases {
		if got := percentile(sorted, c.p); got != c.want {
			t.Errorf("percentile(p%v) = %v, want %v", c.p, got, c.want)
		}
	}
}

func TestPercentileEmptyAndSingle(t *testing.T) {
	if got := percentile(nil, 50); got != 0 {
		t.Errorf("percentile(empty) = %v, want 0", got)
	}
	single := []time.Duration{msDur(7)}
	for _, p := range []float64{50, 90, 95, 99} {
		if got := percentile(single, p); got != msDur(7) {
			t.Errorf("percentile(single, p%v) = %v, want 7ms", p, got)
		}
	}
}

func TestComputeLatencyKnownValues(t *testing.T) {
	// Unsorted input to verify the function sorts internally.
	in := []time.Duration{msDur(40), msDur(10), msDur(30), msDur(20)}
	l := computeLatency(in)

	if l.Count != 4 {
		t.Errorf("Count = %d, want 4", l.Count)
	}
	if l.Min != msDur(10) {
		t.Errorf("Min = %v, want 10ms", l.Min)
	}
	if l.Max != msDur(40) {
		t.Errorf("Max = %v, want 40ms", l.Max)
	}
	// mean = (10+20+30+40)/4 = 25
	if l.Mean != msDur(25) {
		t.Errorf("Mean = %v, want 25ms", l.Mean)
	}
	// nearest-rank on sorted [10,20,30,40]:
	// p50: ceil(0.5*4)-1 = 1 -> 20; p90: ceil(3.6)-1 = 3 -> 40
	if l.P50 != msDur(20) {
		t.Errorf("P50 = %v, want 20ms", l.P50)
	}
	if l.P90 != msDur(40) {
		t.Errorf("P90 = %v, want 40ms", l.P90)
	}
}

func TestComputeLatencyEmpty(t *testing.T) {
	l := computeLatency(nil)
	if l.Count != 0 || l.Min != 0 || l.Max != 0 || l.Mean != 0 {
		t.Errorf("empty latency = %+v, want zero value", l)
	}
}

func TestSummarizeCountsHistogramThroughput(t *testing.T) {
	samples := []Sample{
		{Seq: 0, StatusCode: 200, Total: msDur(10), TTFB: msDur(5), TTLB: msDur(10)},
		{Seq: 1, StatusCode: 200, Total: msDur(20), TTFB: msDur(8), TTLB: msDur(20)},
		{Seq: 2, StatusCode: 500, Total: msDur(30), TTFB: msDur(9), TTLB: msDur(30)},
		{Seq: 3, StatusCode: 404, Total: msDur(40), TTFB: msDur(7), TTLB: msDur(40)},
		{Seq: 4, Err: errors.New("connection refused")},
		{Seq: 5, Err: context.DeadlineExceeded, Timeout: true},
	}
	wall := 2 * time.Second
	s := Summarize(samples, wall)

	if s.Total != 6 {
		t.Errorf("Total = %d, want 6", s.Total)
	}
	if s.Success != 2 {
		t.Errorf("Success = %d, want 2", s.Success)
	}
	if s.Failed != 4 {
		t.Errorf("Failed = %d, want 4", s.Failed)
	}
	if s.Timeouts != 1 {
		t.Errorf("Timeouts = %d, want 1", s.Timeouts)
	}
	if s.Errors != 1 {
		t.Errorf("Errors = %d, want 1", s.Errors)
	}
	// throughput = 6 / 2s = 3 req/s
	if s.Throughput != 3 {
		t.Errorf("Throughput = %v, want 3", s.Throughput)
	}

	wantHist := map[string]int{"200": 2, "500": 1, "404": 1, "error": 1, "timeout": 1}
	for k, v := range wantHist {
		if s.StatusCodes[k] != v {
			t.Errorf("StatusCodes[%q] = %d, want %d", k, s.StatusCodes[k], v)
		}
	}

	// Only the 4 timed (HTTP-responded) samples feed latency stats.
	if s.TotalStats.Count != 4 {
		t.Errorf("TotalStats.Count = %d, want 4 (errors excluded)", s.TotalStats.Count)
	}
	if s.TotalStats.Min != msDur(10) || s.TotalStats.Max != msDur(40) {
		t.Errorf("TotalStats min/max = %v/%v, want 10ms/40ms", s.TotalStats.Min, s.TotalStats.Max)
	}
}

func TestSummarizeEmpty(t *testing.T) {
	s := Summarize(nil, time.Second)
	if s.Total != 0 || s.Success != 0 || s.Failed != 0 {
		t.Errorf("empty summary counts non-zero: %+v", s)
	}
	if s.Throughput != 0 {
		t.Errorf("Throughput = %v, want 0", s.Throughput)
	}
	if s.TotalStats.Count != 0 {
		t.Errorf("TotalStats.Count = %d, want 0", s.TotalStats.Count)
	}
}

func TestCollector(t *testing.T) {
	c := NewCollector()
	if c.Len() != 0 {
		t.Fatalf("new collector Len = %d, want 0", c.Len())
	}
	c.Add(Sample{StatusCode: 200, Total: msDur(5), TTFB: msDur(2), TTLB: msDur(5)})
	c.Add(Sample{StatusCode: 200, Total: msDur(7), TTFB: msDur(3), TTLB: msDur(7)})
	if c.Len() != 2 {
		t.Fatalf("Len = %d, want 2", c.Len())
	}
	s := c.Summarize(time.Second)
	if s.Success != 2 {
		t.Errorf("Success = %d, want 2", s.Success)
	}
}
