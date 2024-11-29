package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/example/uptime/internal/monitor"
)

// runStoreSuite exercises the Store contract against any implementation.
func runStoreSuite(t *testing.T, st Store) {
	t.Helper()
	ctx := context.Background()
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	m := monitor.Monitor{
		ID: "m1", Name: "M1", Kind: monitor.KindHTTP, Target: "https://x/", Method: "GET",
		Interval: 30 * time.Second, Timeout: 5 * time.Second, LatencyThreshold: 800 * time.Millisecond,
		ExpectStatus:   []monitor.StatusRange{{Lo: 200, Hi: 299}, {Lo: 500, Hi: 500}},
		AlertThreshold: 2, Enabled: true,
	}
	if err := st.UpsertMonitor(ctx, m); err != nil {
		t.Fatal(err)
	}

	// Upsert again (update) and confirm a single row with new name.
	m.Name = "M1-updated"
	if err := st.UpsertMonitor(ctx, m); err != nil {
		t.Fatal(err)
	}
	list, err := st.ListMonitors(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "M1-updated" {
		t.Fatalf("list = %+v", list)
	}
	if list[0].Method != "GET" || list[0].AlertThreshold != 2 || len(list[0].ExpectStatus) != 2 {
		t.Fatalf("monitor round-trip lost fields: %+v", list[0])
	}

	// Save a sequence of results.
	for i, s := range []monitor.Status{monitor.StatusUp, monitor.StatusDown, monitor.StatusUp} {
		r := monitor.CheckResult{
			MonitorID:  "m1",
			Timestamp:  base.Add(time.Duration(i) * time.Minute),
			Status:     s,
			Latency:    time.Duration(100+i) * time.Millisecond,
			StatusCode: 200,
		}
		if err := st.SaveResult(ctx, r); err != nil {
			t.Fatal(err)
		}
	}

	// Range query returns only in-window rows, ordered ascending.
	got, err := st.History(ctx, "m1", base, base.Add(90*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 rows in window, got %d", len(got))
	}
	if got[0].Timestamp.After(got[1].Timestamp) {
		t.Fatal("history not ordered by timestamp ascending")
	}

	// LatestStatus reflects last write (Up at t+2m).
	ls, err := st.LatestStatus(ctx, "m1")
	if err != nil {
		t.Fatal(err)
	}
	if ls != monitor.StatusUp {
		t.Fatalf("latest status = %v, want up", ls)
	}

	// Unknown monitor -> StatusUnknown, no error.
	ls, err = st.LatestStatus(ctx, "nope")
	if err != nil || ls != monitor.StatusUnknown {
		t.Fatalf("unknown latest = %v, err = %v", ls, err)
	}

	// Delete removes monitor and results.
	if err := st.DeleteMonitor(ctx, "m1"); err != nil {
		t.Fatal(err)
	}
	list, _ = st.ListMonitors(ctx)
	if len(list) != 0 {
		t.Fatalf("expected no monitors after delete, got %d", len(list))
	}
}

func TestMemoryStore(t *testing.T) {
	st := NewMemory()
	defer st.Close()
	runStoreSuite(t, st)
}

func TestSQLiteStore(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "test.db")
	st, err := OpenSQLite(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	runStoreSuite(t, st)
}

func TestSQLitePersistsAcrossReopen(t *testing.T) {
	dsn := filepath.Join(t.TempDir(), "persist.db")
	ctx := context.Background()

	st, err := OpenSQLite(dsn)
	if err != nil {
		t.Fatal(err)
	}
	r := monitor.CheckResult{MonitorID: "m", Timestamp: time.Now().UTC(), Status: monitor.StatusUp, Latency: time.Second}
	if err := st.SaveResult(ctx, r); err != nil {
		t.Fatal(err)
	}
	st.Close()

	st2, err := OpenSQLite(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
	ls, err := st2.LatestStatus(ctx, "m")
	if err != nil {
		t.Fatal(err)
	}
	if ls != monitor.StatusUp {
		t.Fatalf("status did not survive reopen: %v", ls)
	}
}
