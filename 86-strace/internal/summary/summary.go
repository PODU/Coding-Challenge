// Package summary builds the -c table of call counts, time and errors.
package summary

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type stat struct {
	calls  int
	errors int
	total  time.Duration
}

// Aggregator collects the per-syscall numbers for the -c table.
type Aggregator struct {
	stats map[string]*stat
}

// New returns an empty Aggregator.
func New() *Aggregator {
	return &Aggregator{stats: map[string]*stat{}}
}

// Record counts one call. errno is non-zero if it failed.
func (a *Aggregator) Record(name string, dur time.Duration, errno int) {
	s := a.stats[name]
	if s == nil {
		s = &stat{}
		a.stats[name] = s
	}
	s.calls++
	s.total += dur
	if errno != 0 {
		s.errors++
	}
}

// Render returns the table, most time-consuming syscalls first.
func (a *Aggregator) Render() string {
	type row struct {
		name string
		st   stat
	}
	rows := make([]row, 0, len(a.stats))
	var totalDur time.Duration
	var totalCalls, totalErrors int
	for name, s := range a.stats {
		rows = append(rows, row{name: name, st: *s})
		totalDur += s.total
		totalCalls += s.calls
		totalErrors += s.errors
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].st.total != rows[j].st.total {
			return rows[i].st.total > rows[j].st.total
		}
		return rows[i].name < rows[j].name
	})

	var b strings.Builder
	const (
		header = "%6s %11s %11s %9s %9s %s\n"
		sep    = "------ ----------- ----------- --------- --------- ----------------\n"
	)
	fmt.Fprintf(&b, header, "% time", "seconds", "usecs/call", "calls", "errors", "syscall")
	b.WriteString(sep)

	for _, r := range rows {
		pct := 0.0
		if totalDur > 0 {
			pct = float64(r.st.total) / float64(totalDur) * 100
		}
		usecs := int64(0)
		if r.st.calls > 0 {
			usecs = r.st.total.Microseconds() / int64(r.st.calls)
		}
		errStr := ""
		if r.st.errors > 0 {
			errStr = fmt.Sprintf("%d", r.st.errors)
		}
		fmt.Fprintf(&b, "%6.2f %11.6f %11d %9d %9s %s\n",
			pct, r.st.total.Seconds(), usecs, r.st.calls, errStr, r.name)
	}

	b.WriteString(sep)
	totErrStr := ""
	if totalErrors > 0 {
		totErrStr = fmt.Sprintf("%d", totalErrors)
	}
	fmt.Fprintf(&b, "%6.2f %11.6f %11s %9d %9s %s\n",
		100.0, totalDur.Seconds(), "", totalCalls, totErrStr, "total")
	return b.String()
}
