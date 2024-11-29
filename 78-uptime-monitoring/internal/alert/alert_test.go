package alert

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/smtp"
	"sync"
	"testing"
	"time"

	"github.com/example/uptime/internal/monitor"
)

// RecordingAlerter captures events for assertions — no network, no SMTP.
type RecordingAlerter struct {
	mu     sync.Mutex
	Events []Event
}

func (r *RecordingAlerter) Notify(_ context.Context, ev Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Events = append(r.Events, ev)
	return nil
}

func (r *RecordingAlerter) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.Events)
}

func result(status monitor.Status) monitor.CheckResult {
	return monitor.CheckResult{MonitorID: "m", Status: status, Timestamp: time.Now()}
}

func feed(tr *StateTracker, rec *RecordingAlerter, statuses ...monitor.Status) {
	for _, s := range statuses {
		if ev, fire := tr.Observe(result(s)); fire {
			_ = rec.Notify(context.Background(), ev)
		}
	}
}

func TestStateTracker_FiresOnceAtThreshold(t *testing.T) {
	tr := NewStateTracker(3)
	rec := &RecordingAlerter{}

	feed(tr, rec, monitor.StatusDown, monitor.StatusDown) // below threshold
	if rec.Count() != 0 {
		t.Fatalf("expected no alert below threshold, got %d", rec.Count())
	}
	feed(tr, rec, monitor.StatusDown) // 3rd failure -> exactly one DownEvent
	if rec.Count() != 1 {
		t.Fatalf("expected 1 alert at threshold, got %d", rec.Count())
	}
	if rec.Events[0].Type != EventDown {
		t.Fatalf("expected DownEvent, got %v", rec.Events[0].Type)
	}

	feed(tr, rec, monitor.StatusDown, monitor.StatusDown) // debounce: stay down
	if rec.Count() != 1 {
		t.Fatalf("expected debounce (still 1), got %d", rec.Count())
	}
}

func TestStateTracker_RecoveryFiresOnceAndResets(t *testing.T) {
	tr := NewStateTracker(2)
	rec := &RecordingAlerter{}

	feed(tr, rec, monitor.StatusDown, monitor.StatusDown) // down event
	if rec.Count() != 1 || rec.Events[0].Type != EventDown {
		t.Fatalf("expected one down event, got %+v", rec.Events)
	}
	feed(tr, rec, monitor.StatusUp) // recovery
	if rec.Count() != 2 || rec.Events[1].Type != EventRecovery {
		t.Fatalf("expected recovery event, got %+v", rec.Events)
	}
	feed(tr, rec, monitor.StatusUp, monitor.StatusUp) // staying up: no events
	if rec.Count() != 2 {
		t.Fatalf("expected no further events, got %d", rec.Count())
	}
}

func TestStateTracker_AntiFlapBelowThreshold(t *testing.T) {
	tr := NewStateTracker(3)
	rec := &RecordingAlerter{}
	// up/down/up/down rapidly, never 3 consecutive downs.
	feed(tr, rec,
		monitor.StatusDown, monitor.StatusUp,
		monitor.StatusDown, monitor.StatusDown, monitor.StatusUp,
		monitor.StatusDown, monitor.StatusUp)
	if rec.Count() != 0 {
		t.Fatalf("expected zero alerts for flapping below threshold, got %d", rec.Count())
	}
}

func TestStateTracker_FirstUpNoSpuriousEvent(t *testing.T) {
	tr := NewStateTracker(2)
	rec := &RecordingAlerter{}
	feed(tr, rec, monitor.StatusUp)
	if rec.Count() != 0 {
		t.Fatalf("first-ever Up should not emit an event, got %d", rec.Count())
	}
}

func TestStateTracker_SeedPreventsFalseRecovery(t *testing.T) {
	tr := NewStateTracker(2)
	rec := &RecordingAlerter{}
	tr.Seed("m", monitor.StatusUp) // restart seeded as up
	feed(tr, rec, monitor.StatusUp)
	if rec.Count() != 0 {
		t.Fatalf("seeded up + up should not fire, got %d", rec.Count())
	}
	// Seeded down then up should produce a recovery.
	tr.Seed("m", monitor.StatusDown)
	feed(tr, rec, monitor.StatusUp)
	if rec.Count() != 1 || rec.Events[0].Type != EventRecovery {
		t.Fatalf("expected recovery from seeded-down, got %+v", rec.Events)
	}
}

func TestStateTracker_PerMonitorThreshold(t *testing.T) {
	tr := NewStateTracker(5)
	tr.SetThreshold("m", 1) // override: fail fast
	rec := &RecordingAlerter{}
	feed(tr, rec, monitor.StatusDown)
	if rec.Count() != 1 {
		t.Fatalf("override threshold=1 should fire on first failure, got %d", rec.Count())
	}
}

func TestWebhookAlerter(t *testing.T) {
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	a := NewWebhookAlerter(srv.URL, srv.Client())
	ev := Event{MonitorID: "m", Type: EventDown, Status: monitor.StatusDown, Time: time.Now()}
	if err := a.Notify(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("webhook body not JSON: %v (%s)", err, body)
	}
	if got["monitor_id"] != "m" || got["type"] != "down" || got["status"] != "down" {
		t.Fatalf("unexpected webhook payload: %v", got)
	}
}

func TestEmailAlerter_UsesInjectedSend(t *testing.T) {
	var gotTo []string
	var gotMsg []byte
	a := NewEmailAlerter("smtp:25", "from@x", []string{"to@x"}, nil)
	a.Send = func(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
		gotTo = to
		gotMsg = msg
		return nil
	}
	ev := Event{MonitorID: "m", Type: EventDown, Status: monitor.StatusDown, Time: time.Now()}
	if err := a.Notify(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	if len(gotTo) != 1 || gotTo[0] != "to@x" {
		t.Fatalf("recipients = %v", gotTo)
	}
	if len(gotMsg) == 0 {
		t.Fatal("expected a non-empty message body")
	}
}

func TestMultiAlerterFansOut(t *testing.T) {
	a, b := &RecordingAlerter{}, &RecordingAlerter{}
	m := NewMultiAlerter(a, b)
	_ = m.Notify(context.Background(), Event{MonitorID: "m", Type: EventDown})
	if a.Count() != 1 || b.Count() != 1 {
		t.Fatalf("expected both alerters to receive, got %d and %d", a.Count(), b.Count())
	}
}
