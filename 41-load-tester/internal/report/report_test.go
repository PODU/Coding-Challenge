package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"loadtester/internal/stats"
)

func sampleSummary() stats.Summary {
	mk := func(min int) stats.LatencyStats {
		return stats.LatencyStats{
			Count: 3,
			Min:   time.Duration(min) * time.Millisecond,
			Mean:  15 * time.Millisecond,
			P50:   14 * time.Millisecond,
			P90:   26 * time.Millisecond,
			P95:   33 * time.Millisecond,
			P99:   61 * time.Millisecond,
			Max:   98 * time.Millisecond,
		}
	}
	return stats.Summary{
		Total:       10,
		Success:     8,
		Failed:      2,
		Timeouts:    1,
		Errors:      1,
		WallClock:   4210 * time.Millisecond,
		Throughput:  237.5,
		StatusCodes: map[string]int{"200": 8, "500": 1, "timeout": 1},
		TotalStats:  mk(3),
		TTFBStats:   mk(2),
		TTLBStats:   mk(3),
	}
}

func TestTableRender(t *testing.T) {
	var buf bytes.Buffer
	if err := Table(&buf, sampleSummary()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"Total requests:", "10",
		"Success (2XX):", "8",
		"Failed:", "2",
		"Throughput:", "237.5",
		"Status codes:", "200:", "500:", "timeout:",
		"Latencies (ms):", "Total:", "Time to first byte:", "Time to last byte:",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("table output missing %q\n---\n%s", want, out)
		}
	}
}

func TestTableNAForEmptyLatency(t *testing.T) {
	s := stats.Summarize(nil, time.Second)
	var buf bytes.Buffer
	if err := Table(&buf, s); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "n/a") {
		t.Errorf("expected n/a for empty latency stats, got:\n%s", buf.String())
	}
}

func TestStatusCodeOrdering(t *testing.T) {
	m := map[string]int{"500": 1, "200": 8, "404": 2, "timeout": 1, "error": 1}
	got := sortedCounts(m)
	wantOrder := []string{"200", "404", "500", "error", "timeout"}
	if len(got) != len(wantOrder) {
		t.Fatalf("got %d entries, want %d", len(got), len(wantOrder))
	}
	for i, w := range wantOrder {
		if got[i].key != w {
			t.Errorf("position %d = %q, want %q", i, got[i].key, w)
		}
	}
}

func TestJSONRender(t *testing.T) {
	var buf bytes.Buffer
	if err := JSON(&buf, sampleSummary()); err != nil {
		t.Fatal(err)
	}
	var decoded jsonSummary
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if decoded.Total != 10 || decoded.Success != 8 || decoded.Failed != 2 {
		t.Errorf("decoded counts = %+v, want 10/8/2", decoded)
	}
	if decoded.ThroughputRPS != 237.5 {
		t.Errorf("throughput = %v, want 237.5", decoded.ThroughputRPS)
	}
	if decoded.TotalLatency.Max != 98.0 {
		t.Errorf("total max_ms = %v, want 98.0", decoded.TotalLatency.Max)
	}
}
