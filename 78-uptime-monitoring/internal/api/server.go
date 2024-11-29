// Package api serves the JSON API and the embedded dashboard page.
package api

import (
	"context"
	"embed"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/example/uptime/internal/aggregate"
	"github.com/example/uptime/internal/alert"
	"github.com/example/uptime/internal/monitor"
	"github.com/example/uptime/internal/scheduler"
	"github.com/example/uptime/internal/store"
)

//go:embed dashboard.html
var dashboardFS embed.FS

// Server bundles the dependencies the HTTP handlers need.
type Server struct {
	Store   store.Store
	Tracker *alert.StateTracker
	Metrics *scheduler.Metrics
	Logger  *slog.Logger
}

// New builds a Server.
func New(st store.Store, tracker *alert.StateTracker, m *scheduler.Metrics, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	if m == nil {
		m = &scheduler.Metrics{}
	}
	return &Server{Store: st, Tracker: tracker, Metrics: m, Logger: log}
}

// Handler returns the http.Handler with all routes registered.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/monitors", s.handleListMonitors)
	mux.HandleFunc("POST /api/monitors", s.handleCreateMonitor)
	mux.HandleFunc("DELETE /api/monitors/{id}", s.handleDeleteMonitor)
	mux.HandleFunc("GET /api/monitors/{id}/history", s.handleHistory)
	mux.HandleFunc("GET /api/monitors/{id}/uptime", s.handleUptime)
	mux.HandleFunc("GET /api/export", s.handleExport)
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /metrics", s.handleMetrics)
	mux.HandleFunc("GET /", s.handleDashboard)
	return mux
}

// monitorView is the JSON shape returned for a monitor including live status.
type monitorView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Target      string `json:"target"`
	Method      string `json:"method,omitempty"`
	IntervalS   int    `json:"interval_s"`
	Status      string `json:"status"`
	Enabled     bool   `json:"enabled"`
	AlertThresh int    `json:"alert_threshold"`
}

func (s *Server) handleListMonitors(w http.ResponseWriter, r *http.Request) {
	monitors, err := s.Store.ListMonitors(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	views := make([]monitorView, 0, len(monitors))
	for _, m := range monitors {
		views = append(views, monitorView{
			ID:          m.ID,
			Name:        m.Name,
			Kind:        m.Kind.String(),
			Target:      m.Target,
			Method:      m.Method,
			IntervalS:   int(m.Interval / time.Second),
			Status:      s.status(r.Context(), m.ID),
			Enabled:     m.Enabled,
			AlertThresh: m.AlertThreshold,
		})
	}
	writeJSON(w, http.StatusOK, views)
}

func (s *Server) status(ctx context.Context, id string) string {
	if s.Tracker != nil {
		if st := s.Tracker.Status(id); st != monitor.StatusUnknown {
			return st.String()
		}
	}
	st, err := s.Store.LatestStatus(ctx, id)
	if err != nil {
		return monitor.StatusUnknown.String()
	}
	return st.String()
}

// monitorCreate is the JSON body accepted by POST /api/monitors.
type monitorCreate struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	Target         string `json:"target"`
	Method         string `json:"method"`
	IntervalS      int    `json:"interval_s"`
	TimeoutMs      int    `json:"timeout_ms"`
	AlertThreshold int    `json:"alert_threshold"`
}

func (in monitorCreate) toMonitor() (monitor.Monitor, error) {
	if in.ID == "" || in.Target == "" {
		return monitor.Monitor{}, fmt.Errorf("id and target are required")
	}
	kind, err := monitor.ParseKind(in.Kind)
	if err != nil {
		return monitor.Monitor{}, err
	}
	m := monitor.Monitor{
		ID:             in.ID,
		Name:           in.Name,
		Kind:           kind,
		Target:         in.Target,
		Method:         in.Method,
		Interval:       time.Duration(in.IntervalS) * time.Second,
		Timeout:        time.Duration(in.TimeoutMs) * time.Millisecond,
		AlertThreshold: in.AlertThreshold,
		Enabled:        true,
	}
	if m.Name == "" {
		m.Name = m.ID
	}
	if m.Interval <= 0 {
		m.Interval = 60 * time.Second
	}
	if m.Timeout <= 0 || m.Timeout >= m.Interval {
		m.Timeout = m.Interval / 2
	}
	if m.AlertThreshold <= 0 {
		m.AlertThreshold = 3
	}
	if m.Kind == monitor.KindHTTP && m.Method == "" {
		m.Method = "HEAD"
	}
	return m, nil
}

func (s *Server) handleCreateMonitor(w http.ResponseWriter, r *http.Request) {
	var in monitorCreate
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	m, err := in.toMonitor()
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.Store.UpsertMonitor(r.Context(), m); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": m.ID})
}

func (s *Server) handleDeleteMonitor(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.Store.DeleteMonitor(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	from, to := parseRange(r)
	bucket := parseDuration(r.URL.Query().Get("bucket"), time.Hour)
	results, err := s.Store.History(r.Context(), id, from, to)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	buckets := aggregate.History(results, from, to, bucket)
	writeJSON(w, http.StatusOK, buckets)
}

func (s *Server) handleUptime(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	from, to := parseRange(r)
	results, err := s.Store.History(r.Context(), id, from, to)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	u := aggregate.UptimePercent(results, from, to)
	// JSON can't represent NaN; emit null for an empty window.
	resp := map[string]any{"total": u.Total, "up": u.Up, "down": u.Down}
	if math.IsNaN(u.Percent) {
		resp["percent"] = nil
	} else {
		resp["percent"] = u.Percent
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}
	from, to := parseRange(r)
	monitors, err := s.Store.ListMonitors(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	var all []monitor.CheckResult
	for _, m := range monitors {
		rs, err := s.Store.History(r.Context(), m.ID, from, to)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		all = append(all, rs...)
	}

	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", `attachment; filename="uptime-export.csv"`)
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"monitor_id", "timestamp", "status", "latency_ms", "ttfb_ms", "status_code", "err"})
		for _, r := range all {
			_ = cw.Write([]string{
				r.MonitorID,
				r.Timestamp.UTC().Format(time.RFC3339),
				r.Status.String(),
				strconv.FormatInt(r.Latency.Milliseconds(), 10),
				strconv.FormatInt(r.TTFB.Milliseconds(), 10),
				strconv.Itoa(r.StatusCode),
				r.Err,
			})
		}
		cw.Flush()
	default:
		writeJSON(w, http.StatusOK, all)
	}
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	m := s.Metrics
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "# HELP uptime_checks_total Total checks performed.\n")
	fmt.Fprintf(w, "uptime_checks_total %d\n", m.ChecksTotal.Load())
	fmt.Fprintf(w, "uptime_checks_up %d\n", m.ChecksUp.Load())
	fmt.Fprintf(w, "uptime_checks_down %d\n", m.ChecksDown.Load())
	fmt.Fprintf(w, "uptime_save_errors %d\n", m.SaveErrors.Load())
	fmt.Fprintf(w, "uptime_ticks_skipped %d\n", m.TicksSkipped.Load())
	fmt.Fprintf(w, "uptime_alerts_fired %d\n", m.AlertsFired.Load())
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	b, err := dashboardFS.ReadFile("dashboard.html")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}

func parseRange(r *http.Request) (from, to time.Time) {
	q := r.URL.Query()
	to = parseTime(q.Get("to"), time.Now().UTC())
	from = parseTime(q.Get("from"), to.Add(-24*time.Hour))
	return from, to
}

func parseTime(s string, def time.Time) time.Time {
	if s == "" {
		return def
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC()
	}
	if ms, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.UnixMilli(ms).UTC()
	}
	return def
}

func parseDuration(s string, def time.Duration) time.Duration {
	if s == "" {
		return def
	}
	if d, err := time.ParseDuration(s); err == nil && d > 0 {
		return d
	}
	return def
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}
