package source

import (
	"context"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

// Gopsutil reads the real system through gopsutil, which supports Windows,
// Linux and macOS.
type Gopsutil struct{}

func NewGopsutil() *Gopsutil { return &Gopsutil{} }

// Sample reads the host and process counters. A process that can't be read
// is skipped rather than failing the whole sample.
func (g *Gopsutil) Sample(ctx context.Context) (Sample, error) {
	now := time.Now()
	s := Sample{
		Time:   now,
		NumCPU: runtime.NumCPU(),
	}

	if cts, err := cpu.TimesWithContext(ctx, false); err == nil && len(cts) > 0 {
		s.HostCPU = toCPUTimes(cts[0])
	}

	if vm, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		s.Mem = MemStat{
			Total:     vm.Total,
			Used:      vm.Used,
			Free:      vm.Free,
			Available: vm.Available,
		}
	}

	// Fails on Windows, which has no load average.
	if la, err := load.AvgWithContext(ctx); err == nil && la != nil {
		s.LoadAvg = [3]float64{la.Load1, la.Load5, la.Load15}
		s.HasLoad = true
	}

	if up, err := host.UptimeWithContext(ctx); err == nil {
		s.Uptime = time.Duration(up) * time.Second
	}

	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return s, err
	}
	s.Procs = make([]ProcSample, 0, len(procs))
	for _, p := range procs {
		ps, ok := readProc(ctx, p)
		if !ok {
			continue
		}
		s.Procs = append(s.Procs, ps)
	}
	return s, nil
}

// readProc reads one process's counters, tolerating per-field permission
// errors. It returns ok=false only when nothing useful could be read.
func readProc(ctx context.Context, p *process.Process) (ProcSample, bool) {
	ps := ProcSample{PID: int(p.Pid)}

	if ppid, err := p.PpidWithContext(ctx); err == nil {
		ps.PPID = int(ppid)
	}
	if user, err := p.UsernameWithContext(ctx); err == nil {
		ps.User = user
	}
	// Prefer the short name; fall back to the full command line.
	if name, err := p.NameWithContext(ctx); err == nil && name != "" {
		ps.Command = name
	} else if cmd, err := p.CmdlineWithContext(ctx); err == nil {
		ps.Command = cmd
	}
	if ps.Command == "" {
		// Nothing identifiable; still emit so counts are accurate.
		ps.Command = "?"
	}
	if st, err := p.StatusWithContext(ctx); err == nil && len(st) > 0 {
		ps.State = st[0]
	}
	if t, err := p.TimesWithContext(ctx); err == nil && t != nil {
		ps.CPUTime = t.User + t.System
	}
	if mi, err := p.MemoryInfoWithContext(ctx); err == nil && mi != nil {
		ps.RSS = mi.RSS
	}
	if n, err := p.NumThreadsWithContext(ctx); err == nil {
		ps.Threads = int(n)
	}
	return ps, true
}

func toCPUTimes(t cpu.TimesStat) CPUTimes {
	return CPUTimes{
		User:    t.User,
		Nice:    t.Nice,
		System:  t.System,
		Idle:    t.Idle,
		Iowait:  t.Iowait,
		Irq:     t.Irq,
		Softirq: t.Softirq,
		Steal:   t.Steal,
	}
}

func (g *Gopsutil) Close() error { return nil }

var _ ProcessSource = (*Gopsutil)(nil)
