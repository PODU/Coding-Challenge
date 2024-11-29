package store

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/example/uptime/internal/monitor"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (no cgo)
)

// SQLite is a durable Store backed by a SQLite database via the pure-Go
// modernc.org/sqlite driver. Timestamps are stored as unix milliseconds and the
// results table is indexed on (monitor_id, ts) for efficient range scans.
type SQLite struct {
	db *sql.DB
}

// OpenSQLite opens (creating if needed) a SQLite database at dsn and ensures the
// schema exists. Use ":memory:" for an ephemeral DB.
func OpenSQLite(dsn string) (*SQLite, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite handles one writer at a time; keep the pool small and enable WAL
	// for better concurrent reads.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("pragma: %w", err)
	}
	s := &SQLite{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLite) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS monitors (
    id                   TEXT PRIMARY KEY,
    name                 TEXT NOT NULL,
    kind                 INTEGER NOT NULL,
    target               TEXT NOT NULL,
    method               TEXT NOT NULL,
    interval_s           INTEGER NOT NULL,
    timeout_ms           INTEGER NOT NULL,
    latency_threshold_ms INTEGER NOT NULL,
    expect_status        TEXT NOT NULL,
    alert_threshold      INTEGER NOT NULL,
    enabled              INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS results (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    monitor_id  TEXT NOT NULL,
    ts          INTEGER NOT NULL,
    status      INTEGER NOT NULL,
    latency_ms  INTEGER NOT NULL,
    ttfb_ms     INTEGER NOT NULL,
    status_code INTEGER NOT NULL,
    err         TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_results_monitor_ts ON results (monitor_id, ts);
`
	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

func (s *SQLite) SaveResult(ctx context.Context, r monitor.CheckResult) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO results (monitor_id, ts, status, latency_ms, ttfb_ms, status_code, err)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.MonitorID, r.Timestamp.UnixMilli(), int(r.Status),
		r.Latency.Milliseconds(), r.TTFB.Milliseconds(), r.StatusCode, r.Err)
	if err != nil {
		return fmt.Errorf("save result: %w", err)
	}
	return nil
}

func (s *SQLite) History(ctx context.Context, id string, from, to time.Time) ([]monitor.CheckResult, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT ts, status, latency_ms, ttfb_ms, status_code, err
		 FROM results WHERE monitor_id = ? AND ts >= ? AND ts <= ?
		 ORDER BY ts ASC`,
		id, from.UnixMilli(), to.UnixMilli())
	if err != nil {
		return nil, fmt.Errorf("history query: %w", err)
	}
	defer rows.Close()

	var out []monitor.CheckResult
	for rows.Next() {
		var (
			ts, status, latency, ttfb, code int64
			errStr                          string
		)
		if err := rows.Scan(&ts, &status, &latency, &ttfb, &code, &errStr); err != nil {
			return nil, fmt.Errorf("history scan: %w", err)
		}
		out = append(out, monitor.CheckResult{
			MonitorID:  id,
			Timestamp:  time.UnixMilli(ts).UTC(),
			Status:     monitor.Status(status),
			Latency:    time.Duration(latency) * time.Millisecond,
			TTFB:       time.Duration(ttfb) * time.Millisecond,
			StatusCode: int(code),
			Err:        errStr,
		})
	}
	return out, rows.Err()
}

func (s *SQLite) LatestStatus(ctx context.Context, id string) (monitor.Status, error) {
	var status int64
	err := s.db.QueryRowContext(ctx,
		`SELECT status FROM results WHERE monitor_id = ? ORDER BY ts DESC LIMIT 1`, id).
		Scan(&status)
	if err == sql.ErrNoRows {
		return monitor.StatusUnknown, nil
	}
	if err != nil {
		return monitor.StatusUnknown, fmt.Errorf("latest status: %w", err)
	}
	return monitor.Status(status), nil
}

func (s *SQLite) UpsertMonitor(ctx context.Context, m monitor.Monitor) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO monitors
		   (id, name, kind, target, method, interval_s, timeout_ms,
		    latency_threshold_ms, expect_status, alert_threshold, enabled)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   name=excluded.name, kind=excluded.kind, target=excluded.target,
		   method=excluded.method, interval_s=excluded.interval_s,
		   timeout_ms=excluded.timeout_ms,
		   latency_threshold_ms=excluded.latency_threshold_ms,
		   expect_status=excluded.expect_status,
		   alert_threshold=excluded.alert_threshold, enabled=excluded.enabled`,
		m.ID, m.Name, int(m.Kind), m.Target, m.Method,
		int64(m.Interval/time.Second), m.Timeout.Milliseconds(),
		m.LatencyThreshold.Milliseconds(), encodeRanges(m.ExpectStatus),
		m.AlertThreshold, boolToInt(m.Enabled))
	if err != nil {
		return fmt.Errorf("upsert monitor: %w", err)
	}
	return nil
}

func (s *SQLite) DeleteMonitor(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit
	if _, err := tx.ExecContext(ctx, `DELETE FROM results WHERE monitor_id = ?`, id); err != nil {
		return fmt.Errorf("delete results: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM monitors WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete monitor: %w", err)
	}
	return tx.Commit()
}

func (s *SQLite) ListMonitors(ctx context.Context) ([]monitor.Monitor, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, kind, target, method, interval_s, timeout_ms,
		        latency_threshold_ms, expect_status, alert_threshold, enabled
		 FROM monitors ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list monitors: %w", err)
	}
	defer rows.Close()

	var out []monitor.Monitor
	for rows.Next() {
		var (
			m                                          monitor.Monitor
			kind, intervalS, timeoutMs, latMs, enabled int64
			ranges                                     string
		)
		if err := rows.Scan(&m.ID, &m.Name, &kind, &m.Target, &m.Method,
			&intervalS, &timeoutMs, &latMs, &ranges, &m.AlertThreshold, &enabled); err != nil {
			return nil, fmt.Errorf("list scan: %w", err)
		}
		m.Kind = monitor.Kind(kind)
		m.Interval = time.Duration(intervalS) * time.Second
		m.Timeout = time.Duration(timeoutMs) * time.Millisecond
		m.LatencyThreshold = time.Duration(latMs) * time.Millisecond
		m.ExpectStatus = decodeRanges(ranges)
		m.Enabled = enabled != 0
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *SQLite) Close() error { return s.db.Close() }

func encodeRanges(rs []monitor.StatusRange) string {
	parts := make([]string, 0, len(rs))
	for _, r := range rs {
		parts = append(parts, r.String())
	}
	return strings.Join(parts, ",")
}

func decodeRanges(s string) []monitor.StatusRange {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var out []monitor.StatusRange
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if lo, hi, ok := strings.Cut(p, "-"); ok {
			l, _ := strconv.Atoi(strings.TrimSpace(lo))
			h, _ := strconv.Atoi(strings.TrimSpace(hi))
			out = append(out, monitor.StatusRange{Lo: l, Hi: h})
		} else {
			n, _ := strconv.Atoi(p)
			out = append(out, monitor.StatusRange{Lo: n, Hi: n})
		}
	}
	return out
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
