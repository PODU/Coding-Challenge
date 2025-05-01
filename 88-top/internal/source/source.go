// Package source reads raw process and host counters. Everything that talks
// to the OS goes through ProcessSource.
package source

import (
	"context"
	"time"
)

type ProcessSource interface {
	Sample(ctx context.Context) (Sample, error)
	Close() error
}

// Sample is one reading of the counters. They only go up, so the engine can
// work out rates from two samples.
type Sample struct {
	Time    time.Time
	NumCPU  int
	LoadAvg [3]float64 // zero where there is none (Windows)
	HasLoad bool
	Uptime  time.Duration
	Mem     MemStat
	HostCPU CPUTimes
	Procs   []ProcSample
}

// MemStat is physical memory, in bytes.
type MemStat struct {
	Total      uint64
	Used       uint64
	Free       uint64
	Available  uint64
	Wired      uint64
	Compressed uint64
}

// CPUTimes is the total CPU time spent in each state since boot, in seconds.
type CPUTimes struct {
	User    float64
	Nice    float64
	System  float64
	Idle    float64
	Iowait  float64
	Irq     float64
	Softirq float64
	Steal   float64
}

func (c CPUTimes) Total() float64 {
	return c.User + c.Nice + c.System + c.Idle + c.Iowait + c.Irq + c.Softirq + c.Steal
}

// Busy is the time not spent idle or waiting on I/O.
func (c CPUTimes) Busy() float64 {
	return c.Total() - c.Idle - c.Iowait
}

type ProcSample struct {
	PID     int
	PPID    int
	User    string
	Command string
	State   string
	CPUTime float64 // user + system seconds so far
	RSS     uint64  // bytes
	Threads int
}
