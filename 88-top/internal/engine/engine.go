// Package engine turns two raw samples into what the screen shows: %CPU per
// process, the summary header, and the filtered, sorted process list.
package engine

import (
	"strings"

	"github.com/example/cctop/internal/model"
	"github.com/example/cctop/internal/source"
)

// Compute builds the Snapshot for cur. On the first frame prev is the zero
// Sample, and every %CPU comes out as 0.
func Compute(prev, cur source.Sample, view model.ViewState) model.Snapshot {
	hasPrev := !prev.Time.IsZero()
	deltaWall := cur.Time.Sub(prev.Time).Seconds()
	if !hasPrev || deltaWall <= 0 {
		deltaWall = 0
	}

	prevCPU := make(map[int]float64, len(prev.Procs))
	if hasPrev {
		for _, p := range prev.Procs {
			prevCPU[p.PID] = p.CPUTime
		}
	}

	procs := make([]model.Process, 0, len(cur.Procs))
	for _, ps := range cur.Procs {
		if view.UserFilter != "" && ps.User != view.UserFilter {
			continue
		}
		procs = append(procs, model.Process{
			PID:        ps.PID,
			PPID:       ps.PPID,
			User:       ps.User,
			Command:    ps.Command,
			State:      ps.State,
			CPUPercent: procCPUPercent(ps, prevCPU, deltaWall, cur.NumCPU, view.Solaris),
			MemPercent: memPercent(ps.RSS, cur.Mem.Total),
			RSS:        ps.RSS,
			CPUTime:    ps.CPUTime,
			Threads:    ps.Threads,
		})
	}

	sortProcs(procs, view)

	return model.Snapshot{
		Summary: aggregate(prev, cur, deltaWall),
		Procs:   procs,
		View:    view,
	}
}

// procCPUPercent is top's %CPU: CPU time used over the last interval as a
// share of that interval. By default (Irix mode) it can pass 100 on a
// multi-core machine; Solaris mode divides by the number of CPUs.
func procCPUPercent(ps source.ProcSample, prevCPU map[int]float64, deltaWall float64, numCPU int, solaris bool) float64 {
	if deltaWall <= 0 {
		return 0
	}
	prevTime, seen := prevCPU[ps.PID]
	if !seen {
		return 0 // a new process; no rate until the next frame
	}
	// A counter going backwards means the PID was reused.
	delta := max(ps.CPUTime-prevTime, 0)
	pct := 100 * delta / deltaWall
	if solaris && numCPU > 0 {
		pct /= float64(numCPU)
	}
	return pct
}

func memPercent(rss, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return 100 * float64(rss) / float64(total)
}

// aggregate builds the summary header.
func aggregate(prev, cur source.Sample, deltaWall float64) model.Summary {
	s := model.Summary{
		Time:     cur.Time,
		Uptime:   cur.Uptime,
		LoadAvg:  cur.LoadAvg,
		HasLoad:  cur.HasLoad,
		NumCPU:   cur.NumCPU,
		MemTotal: cur.Mem.Total,
		MemUsed:  cur.Mem.Used,
		MemFree:  cur.Mem.Free,
	}

	for _, p := range cur.Procs {
		s.TasksTotal++
		switch normalizeState(p.State) {
		case "R":
			s.Running++
		case "T":
			s.Stopped++
		case "Z":
			s.Zombie++
		default: // top counts D and I as sleeping too
			s.Sleeping++
		}
	}

	s.CPUUser, s.CPUSys, s.CPUIdle = hostCPUPercent(prev.HostCPU, cur.HostCPU, deltaWall)
	return s
}

// hostCPUPercent works out user, system and idle percentages between two
// samples. With nothing to compare it reports the machine as idle.
func hostCPUPercent(prev, cur source.CPUTimes, deltaWall float64) (user, sys, idle float64) {
	total := cur.Total() - prev.Total()
	if deltaWall <= 0 || total <= 0 {
		return 0, 0, 100
	}
	userD := (cur.User - prev.User) + (cur.Nice - prev.Nice)
	sysD := (cur.System - prev.System) + (cur.Irq - prev.Irq) +
		(cur.Softirq - prev.Softirq) + (cur.Steal - prev.Steal)
	idleD := (cur.Idle - prev.Idle) + (cur.Iowait - prev.Iowait)
	return clampPct(100 * userD / total), clampPct(100 * sysD / total), clampPct(100 * idleD / total)
}

func clampPct(p float64) float64 {
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}

// normalizeState maps the status strings different platforms use onto top's
// single letters.
func normalizeState(state string) string {
	if state == "" {
		return "S"
	}
	switch strings.ToLower(state) {
	case "r", "running":
		return "R"
	case "t", "stop", "stopped":
		return "T"
	case "z", "zombie":
		return "Z"
	case "d", "disk", "uninterruptible", "blocked":
		return "D"
	case "i", "idle":
		return "I"
	default:
		return "S"
	}
}
