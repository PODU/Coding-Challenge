package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/example/cctop/internal/model"
)

func goldenSnapshot() model.Snapshot {
	return model.Snapshot{
		Summary: model.Summary{
			Time:       time.Date(2026, 1, 1, 14, 32, 7, 0, time.UTC),
			Uptime:     3*time.Hour + 21*time.Minute,
			LoadAvg:    [3]float64{0.42, 0.55, 0.61},
			HasLoad:    true,
			NumCPU:     8,
			TasksTotal: 2,
			Running:    1,
			Sleeping:   1,
			CPUUser:    6.3,
			CPUSys:     2.1,
			CPUIdle:    91.6,
			MemTotal:   16 * 1024 * 1024 * 1024,
			MemUsed:    9 * 1024 * 1024 * 1024,
			MemFree:    7 * 1024 * 1024 * 1024,
		},
		Procs: []model.Process{
			{PID: 1234, User: "aditya", Command: "chrome", State: "R",
				CPUPercent: 38.4, MemPercent: 4.2, RSS: 512 * 1024 * 1024, CPUTime: 62.44, Threads: 24},
			{PID: 910, User: "root", Command: "systemd", State: "S",
				CPUPercent: 5.1, MemPercent: 0.8, RSS: 96 * 1024 * 1024, CPUTime: 11.07, Threads: 8},
		},
		View: model.ViewState{Sort: model.SortCPU, Unit: model.UnitAuto},
	}
}

func TestProcLineGolden(t *testing.T) {
	p := goldenSnapshot().Procs[0]
	got := ProcLine(p, model.UnitAuto, 100)
	want := "  1234 aditya      38.4   4.2   512.0Mi   1:02.44   24 R chrome"
	if got != want {
		t.Errorf("ProcLine mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestFrameGolden(t *testing.T) {
	snap := goldenSnapshot()
	lines := Frame(snap, 2.0, 100, 24)

	if !strings.HasPrefix(lines[0], "cctop - 14:32:07 up 3:21,  load average: 0.42, 0.55, 0.61") {
		t.Errorf("header line 0 = %q", lines[0])
	}
	if lines[1] != "Tasks: 2 total, 1 running, 1 sleeping, 0 stopped, 0 zombie" {
		t.Errorf("tasks line = %q", lines[1])
	}
	if !strings.Contains(lines[2], "6.3 us") || !strings.Contains(lines[2], "91.6 id") {
		t.Errorf("cpu line = %q", lines[2])
	}
	if !strings.Contains(lines[3], "16.0Gi total") {
		t.Errorf("mem line = %q", lines[3])
	}
	// Column header is line index 5.
	if !strings.Contains(lines[5], "PID") || !strings.Contains(lines[5], "COMMAND") {
		t.Errorf("column header = %q", lines[5])
	}
	// Two process rows follow, then a status line.
	if !strings.Contains(lines[6], "chrome") {
		t.Errorf("row 0 = %q", lines[6])
	}
	if !strings.Contains(lines[7], "systemd") {
		t.Errorf("row 1 = %q", lines[7])
	}
	last := lines[len(lines)-1]
	if !strings.Contains(last, "Sort: %CPU(desc)") || !strings.Contains(last, "[q]uit") {
		t.Errorf("status line = %q", last)
	}
}

func TestFrameClipsToHeight(t *testing.T) {
	snap := goldenSnapshot()
	// height 8 leaves room for 6 header lines + 1 status = 7, so only 1 proc row.
	lines := Frame(snap, 2.0, 100, 8)
	if len(lines) != 8 {
		t.Fatalf("expected 8 lines, got %d", len(lines))
	}
}

func TestLoadAverageDash(t *testing.T) {
	snap := goldenSnapshot()
	snap.Summary.HasLoad = false
	lines := Frame(snap, 2.0, 100, 24)
	if !strings.Contains(lines[0], "load average: -") {
		t.Errorf("expected dash for missing load, got %q", lines[0])
	}
}
