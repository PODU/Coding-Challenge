// Package alert turns check results into notifications that fire only when a
// monitor actually goes down or recovers, plus the alerters that send them.
package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"github.com/example/uptime/internal/monitor"
)

// EventType distinguishes a confirmed outage from a recovery.
type EventType int

const (
	// EventDown is emitted on a confirmed UP/UNKNOWN -> DOWN transition.
	EventDown EventType = iota
	// EventRecovery is emitted on a confirmed DOWN -> UP transition.
	EventRecovery
)

func (e EventType) String() string {
	if e == EventRecovery {
		return "recovery"
	}
	return "down"
}

// Event is a confirmed state transition for a monitor.
type Event struct {
	MonitorID        string              `json:"monitor_id"`
	Type             EventType           `json:"type"`
	Status           monitor.Status      `json:"status"`
	Time             time.Time           `json:"time"`
	ConsecutiveFails int                 `json:"consecutive_fails"`
	Result           monitor.CheckResult `json:"result"`
}

// MarshalJSON renders Type/Status as strings for a friendlier API/webhook payload.
func (e Event) MarshalJSON() ([]byte, error) {
	type alias Event
	return json.Marshal(struct {
		alias
		TypeStr   string `json:"type"`
		StatusStr string `json:"status"`
	}{alias(e), e.Type.String(), e.Status.String()})
}

// Alerter delivers an Event to a destination.
type Alerter interface {
	Notify(ctx context.Context, ev Event) error
}

// StateTracker decides when to alert. A monitor is declared down after
// `threshold` consecutive failures, which fires one DOWN event; the next success
// fires one recovery and resets the count. Safe for concurrent use.
type StateTracker struct {
	mu               sync.Mutex
	defaultThreshold int
	thresholds       map[string]int
	states           map[string]*monState
}

type monState struct {
	status monitor.Status
	fails  int
}

// NewStateTracker builds a tracker. defaultThreshold (min 1) is the consecutive
// failure count required before a DOWN alert when a monitor has no override.
func NewStateTracker(defaultThreshold int) *StateTracker {
	if defaultThreshold < 1 {
		defaultThreshold = 1
	}
	return &StateTracker{
		defaultThreshold: defaultThreshold,
		thresholds:       make(map[string]int),
		states:           make(map[string]*monState),
	}
}

// SetThreshold overrides the failure threshold for a single monitor.
func (t *StateTracker) SetThreshold(monitorID string, n int) {
	if n < 1 {
		n = 1
	}
	t.mu.Lock()
	t.thresholds[monitorID] = n
	t.mu.Unlock()
}

// Seed sets the known current status for a monitor without emitting an event.
// Used on restart so the first post-restart probe doesn't false-alert.
func (t *StateTracker) Seed(monitorID string, status monitor.Status) {
	t.mu.Lock()
	t.states[monitorID] = &monState{status: status}
	t.mu.Unlock()
}

// Status returns the tracker's current confirmed status for a monitor.
func (t *StateTracker) Status(monitorID string) monitor.Status {
	t.mu.Lock()
	defer t.mu.Unlock()
	if s, ok := t.states[monitorID]; ok {
		return s.status
	}
	return monitor.StatusUnknown
}

// Observe records a check result and returns the event to send, if any.
func (t *StateTracker) Observe(r monitor.CheckResult) (Event, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	st, ok := t.states[r.MonitorID]
	if !ok {
		st = &monState{status: monitor.StatusUnknown}
		t.states[r.MonitorID] = st
	}
	threshold := t.defaultThreshold
	if n, ok := t.thresholds[r.MonitorID]; ok {
		threshold = n
	}

	if r.Status == monitor.StatusUp {
		wasDown := st.status == monitor.StatusDown
		st.fails = 0
		st.status = monitor.StatusUp
		if wasDown {
			return Event{
				MonitorID: r.MonitorID,
				Type:      EventRecovery,
				Status:    monitor.StatusUp,
				Time:      r.Timestamp,
				Result:    r,
			}, true
		}
		return Event{}, false
	}

	st.fails++
	if st.status != monitor.StatusDown && st.fails >= threshold {
		st.status = monitor.StatusDown
		return Event{
			MonitorID:        r.MonitorID,
			Type:             EventDown,
			Status:           monitor.StatusDown,
			Time:             r.Timestamp,
			ConsecutiveFails: st.fails,
			Result:           r,
		}, true
	}
	return Event{}, false
}

// LogAlerter writes events to a slog.Logger. Always safe, no network.
type LogAlerter struct{ Logger *slog.Logger }

// NewLogAlerter returns a LogAlerter using the given logger (defaults to slog default).
func NewLogAlerter(l *slog.Logger) *LogAlerter {
	if l == nil {
		l = slog.Default()
	}
	return &LogAlerter{Logger: l}
}

func (a *LogAlerter) Notify(_ context.Context, ev Event) error {
	level := slog.LevelError
	if ev.Type == EventRecovery {
		level = slog.LevelInfo
	}
	a.Logger.Log(context.Background(), level, "monitor "+ev.Type.String(),
		"monitor", ev.MonitorID, "status", ev.Status.String(),
		"consecutive_fails", ev.ConsecutiveFails, "err", ev.Result.Err)
	return nil
}

// WebhookAlerter POSTs the event as JSON to a URL.
type WebhookAlerter struct {
	URL    string
	Client *http.Client
}

// NewWebhookAlerter returns a WebhookAlerter; a nil client uses a 10s-timeout default.
func NewWebhookAlerter(url string, client *http.Client) *WebhookAlerter {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &WebhookAlerter{URL: url, Client: client}
}

func (a *WebhookAlerter) Notify(ctx context.Context, ev Event) error {
	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned %s", resp.Status)
	}
	return nil
}

// SendMailFunc has the signature of net/smtp.SendMail, so tests can swap it.
type SendMailFunc func(addr string, a smtp.Auth, from string, to []string, msg []byte) error

// EmailAlerter delivers events over SMTP.
type EmailAlerter struct {
	Addr string // host:port
	From string
	To   []string
	Auth smtp.Auth
	Send SendMailFunc
}

// NewEmailAlerter returns an EmailAlerter using net/smtp.SendMail by default.
func NewEmailAlerter(addr, from string, to []string, auth smtp.Auth) *EmailAlerter {
	return &EmailAlerter{Addr: addr, From: from, To: to, Auth: auth, Send: smtp.SendMail}
}

func (a *EmailAlerter) Notify(_ context.Context, ev Event) error {
	send := a.Send
	if send == nil {
		send = smtp.SendMail
	}
	subject := fmt.Sprintf("[uptime] %s is %s", ev.MonitorID, ev.Status.String())
	var b strings.Builder
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(a.To, ", "))
	fmt.Fprintf(&b, "From: %s\r\n", a.From)
	fmt.Fprintf(&b, "Subject: %s\r\n\r\n", subject)
	fmt.Fprintf(&b, "Monitor %s transitioned to %s at %s.\r\n",
		ev.MonitorID, ev.Status.String(), ev.Time.Format(time.RFC3339))
	if ev.Result.Err != "" {
		fmt.Fprintf(&b, "Error: %s\r\n", ev.Result.Err)
	}
	return send(a.Addr, a.Auth, a.From, a.To, []byte(b.String()))
}

// MultiAlerter fans an event out to several alerters, returning the first error
// but still attempting every alerter.
type MultiAlerter struct{ Alerters []Alerter }

// NewMultiAlerter combines alerters.
func NewMultiAlerter(as ...Alerter) *MultiAlerter { return &MultiAlerter{Alerters: as} }

func (m *MultiAlerter) Notify(ctx context.Context, ev Event) error {
	var firstErr error
	for _, a := range m.Alerters {
		if err := a.Notify(ctx, ev); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
