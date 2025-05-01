package source

import (
	"context"
	"time"
)

// Fake is a made-up ProcessSource for --demo and the tests. Every Sample moves
// the clock and the CPU counters forward by fixed amounts, so the rates come
// out the same every time.
type Fake struct {
	now      time.Time
	step     time.Duration
	numCPU   int
	memTotal uint64
	calls    int
}

// NewFake returns a Fake starting at a fixed epoch, advancing `step` per call.
func NewFake(step time.Duration) *Fake {
	if step <= 0 {
		step = 2 * time.Second
	}
	return &Fake{
		now:      time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		step:     step,
		numCPU:   8,
		memTotal: 16 * 1024 * 1024 * 1024, // 16 GiB
	}
}

// procScript describes how a fake process accrues CPU per second.
type procScript struct {
	pid     int
	user    string
	command string
	state   string
	rss     uint64
	threads int
	cpuPerS float64 // CPU-seconds consumed per wall-clock second
}

var fakeProcs = []procScript{
	{1234, "aditya", "chrome", "R", 512 * 1024 * 1024, 24, 0.40},
	{910, "root", "systemd", "S", 96 * 1024 * 1024, 8, 0.05},
	{2048, "aditya", "go", "R", 256 * 1024 * 1024, 12, 0.25},
	{77, "root", "kthreadd", "S", 4 * 1024 * 1024, 1, 0.01},
	{3001, "postgres", "postgres", "S", 320 * 1024 * 1024, 16, 0.12},
	{4500, "aditya", "code", "S", 700 * 1024 * 1024, 30, 0.18},
}

func (f *Fake) Sample(_ context.Context) (Sample, error) {
	f.calls++
	elapsed := float64(f.calls) * f.step.Seconds() // seconds since epoch
	now := f.now.Add(time.Duration(f.calls) * f.step)

	procs := make([]ProcSample, 0, len(fakeProcs))
	var busy float64
	for _, p := range fakeProcs {
		cpuTime := p.cpuPerS * elapsed
		busy += p.cpuPerS
		procs = append(procs, ProcSample{
			PID:     p.pid,
			PPID:    1,
			User:    p.user,
			Command: p.command,
			State:   p.state,
			CPUTime: cpuTime,
			RSS:     p.rss,
			Threads: p.threads,
		})
	}

	// Host CPU counters: spread `busy` across user/system, rest is idle.
	idlePerS := float64(f.numCPU) - busy
	if idlePerS < 0 {
		idlePerS = 0
	}
	host := CPUTimes{
		User:   busy * 0.7 * elapsed,
		System: busy * 0.3 * elapsed,
		Idle:   idlePerS * elapsed,
	}

	used := uint64(0)
	for _, p := range fakeProcs {
		used += p.rss
	}

	return Sample{
		Time:    now,
		NumCPU:  f.numCPU,
		LoadAvg: [3]float64{0.42, 0.55, 0.61},
		HasLoad: true,
		Uptime:  3*time.Hour + 21*time.Minute,
		Mem: MemStat{
			Total:     f.memTotal,
			Used:      used,
			Free:      f.memTotal - used,
			Available: f.memTotal - used,
		},
		HostCPU: host,
		Procs:   procs,
	}, nil
}

func (f *Fake) Close() error { return nil }

var _ ProcessSource = (*Fake)(nil)
