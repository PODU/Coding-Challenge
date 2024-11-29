package store

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/example/uptime/internal/monitor"
)

// Memory is an in-memory Store used for tests and the "memory" driver. It is
// safe for concurrent use.
type Memory struct {
	mu       sync.RWMutex
	results  map[string][]monitor.CheckResult
	monitors map[string]monitor.Monitor
	order    []string // preserves monitor insertion order
}

// NewMemory returns an empty in-memory store.
func NewMemory() *Memory {
	return &Memory{
		results:  make(map[string][]monitor.CheckResult),
		monitors: make(map[string]monitor.Monitor),
	}
}

func (m *Memory) SaveResult(_ context.Context, r monitor.CheckResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.results[r.MonitorID] = append(m.results[r.MonitorID], r)
	return nil
}

func (m *Memory) History(_ context.Context, id string, from, to time.Time) ([]monitor.CheckResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []monitor.CheckResult
	for _, r := range m.results[id] {
		if (r.Timestamp.Equal(from) || r.Timestamp.After(from)) &&
			(r.Timestamp.Equal(to) || r.Timestamp.Before(to)) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp.Before(out[j].Timestamp) })
	return out, nil
}

func (m *Memory) LatestStatus(_ context.Context, id string) (monitor.Status, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rs := m.results[id]
	if len(rs) == 0 {
		return monitor.StatusUnknown, nil
	}
	latest := rs[0]
	for _, r := range rs[1:] {
		if r.Timestamp.After(latest.Timestamp) {
			latest = r
		}
	}
	return latest.Status, nil
}

func (m *Memory) UpsertMonitor(_ context.Context, mon monitor.Monitor) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.monitors[mon.ID]; !ok {
		m.order = append(m.order, mon.ID)
	}
	m.monitors[mon.ID] = mon
	return nil
}

func (m *Memory) DeleteMonitor(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.monitors, id)
	delete(m.results, id)
	for i, x := range m.order {
		if x == id {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
	return nil
}

func (m *Memory) ListMonitors(_ context.Context) ([]monitor.Monitor, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]monitor.Monitor, 0, len(m.order))
	for _, id := range m.order {
		out = append(out, m.monitors[id])
	}
	return out, nil
}

func (m *Memory) Close() error { return nil }
