package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/example/uptime/internal/alert"
	"github.com/example/uptime/internal/monitor"
	"github.com/example/uptime/internal/store"
)

func seedStore(t *testing.T) (*store.Memory, *alert.StateTracker) {
	t.Helper()
	st := store.NewMemory()
	ctx := context.Background()
	m := monitor.Monitor{
		ID: "m1", Name: "API", Kind: monitor.KindHTTP, Target: "https://x/", Method: "GET",
		Interval: 30 * time.Second, Enabled: true, AlertThreshold: 3,
	}
	if err := st.UpsertMonitor(ctx, m); err != nil {
		t.Fatal(err)
	}
	base := time.Now().UTC().Add(-10 * time.Minute)
	statuses := []monitor.Status{monitor.StatusUp, monitor.StatusUp, monitor.StatusDown, monitor.StatusUp}
	for i, s := range statuses {
		_ = st.SaveResult(ctx, monitor.CheckResult{
			MonitorID: "m1", Timestamp: base.Add(time.Duration(i) * time.Minute),
			Status: s, Latency: time.Duration(100+i*10) * time.Millisecond, StatusCode: 200,
		})
	}
	tr := alert.NewStateTracker(3)
	tr.Seed("m1", monitor.StatusUp)
	return st, tr
}

func doReq(h http.Handler, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func TestListMonitors(t *testing.T) {
	st, tr := seedStore(t)
	h := New(st, tr, nil, nil).Handler()

	rec := doReq(h, "GET", "/api/monitors")
	if rec.Code != 200 {
		t.Fatalf("code = %d", rec.Code)
	}
	var views []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &views); err != nil {
		t.Fatal(err)
	}
	if len(views) != 1 {
		t.Fatalf("expected 1 monitor, got %d", len(views))
	}
	if views[0]["id"] != "m1" || views[0]["status"] != "up" {
		t.Fatalf("monitor view = %v", views[0])
	}
}

func TestHistoryEndpoint(t *testing.T) {
	st, tr := seedStore(t)
	h := New(st, tr, nil, nil).Handler()

	from := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	to := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	rec := doReq(h, "GET", "/api/monitors/m1/history?from="+from+"&to="+to+"&bucket=1m")
	if rec.Code != 200 {
		t.Fatalf("code = %d body=%s", rec.Code, rec.Body)
	}
	var buckets []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &buckets); err != nil {
		t.Fatal(err)
	}
	if len(buckets) == 0 {
		t.Fatal("expected non-empty bucket series")
	}
	total := 0
	for _, b := range buckets {
		total += int(b["total"].(float64))
	}
	if total != 4 {
		t.Fatalf("expected 4 checks across buckets, got %d", total)
	}
}

func TestUptimeEndpoint(t *testing.T) {
	st, tr := seedStore(t)
	h := New(st, tr, nil, nil).Handler()

	from := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	to := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	rec := doReq(h, "GET", "/api/monitors/m1/uptime?from="+from+"&to="+to)
	if rec.Code != 200 {
		t.Fatalf("code = %d", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if int(resp["total"].(float64)) != 4 || int(resp["up"].(float64)) != 3 {
		t.Fatalf("uptime resp = %v", resp)
	}
	if pct := resp["percent"].(float64); pct < 74 || pct > 76 {
		t.Fatalf("percent = %v, want ~75", pct)
	}
}

func TestUptimeEndpoint_EmptyWindowNull(t *testing.T) {
	st, tr := seedStore(t)
	h := New(st, tr, nil, nil).Handler()
	// window far in the past with no data
	rec := doReq(h, "GET", "/api/monitors/m1/uptime?from=2000-01-01T00:00:00Z&to=2000-01-02T00:00:00Z")
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["percent"] != nil {
		t.Fatalf("expected null percent for empty window, got %v", resp["percent"])
	}
}

func TestExportCSV(t *testing.T) {
	st, tr := seedStore(t)
	h := New(st, tr, nil, nil).Handler()
	to := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	rec := doReq(h, "GET", "/api/export?format=csv&from=2000-01-01T00:00:00Z&to="+to)
	if rec.Code != 200 {
		t.Fatalf("code = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Fatalf("content-type = %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "monitor_id,timestamp,status") {
		t.Fatalf("missing CSV header: %s", body)
	}
	// header + 4 rows
	lines := strings.Count(strings.TrimSpace(body), "\n") + 1
	if lines != 5 {
		t.Fatalf("expected 5 CSV lines, got %d:\n%s", lines, body)
	}
}

func TestExportJSON(t *testing.T) {
	st, tr := seedStore(t)
	h := New(st, tr, nil, nil).Handler()
	to := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	rec := doReq(h, "GET", "/api/export?format=json&from=2000-01-01T00:00:00Z&to="+to)
	var rows []monitor.CheckResult
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatalf("expected 4 exported rows, got %d", len(rows))
	}
}

func TestCreateAndDeleteMonitor(t *testing.T) {
	st, tr := seedStore(t)
	h := New(st, tr, nil, nil).Handler()

	body := `{"id":"new","name":"New","kind":"tcp","target":"h:1234","interval_s":15}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/api/monitors", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create code = %d body=%s", rec.Code, rec.Body)
	}
	list, _ := st.ListMonitors(context.Background())
	if len(list) != 2 {
		t.Fatalf("expected 2 monitors after create, got %d", len(list))
	}

	rec = doReq(h, "DELETE", "/api/monitors/new")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete code = %d", rec.Code)
	}
	list, _ = st.ListMonitors(context.Background())
	if len(list) != 1 {
		t.Fatalf("expected 1 monitor after delete, got %d", len(list))
	}
}

func TestHealthzAndMetricsAndDashboard(t *testing.T) {
	st, tr := seedStore(t)
	h := New(st, tr, nil, nil).Handler()

	if rec := doReq(h, "GET", "/healthz"); rec.Code != 200 {
		t.Fatalf("healthz code = %d", rec.Code)
	}
	rec := doReq(h, "GET", "/metrics")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "uptime_checks_total") {
		t.Fatalf("metrics body = %s", rec.Body)
	}
	rec = doReq(h, "GET", "/")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "<title>Uptime Monitor</title>") {
		t.Fatalf("dashboard not served: code=%d", rec.Code)
	}
}
