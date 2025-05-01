package tui

import (
	"fmt"
	"strings"

	"github.com/example/cctop/internal/format"
	"github.com/example/cctop/internal/model"
)

const (
	wPID  = 6
	wUser = 10
	wCPU  = 5
	wMem  = 5
	wRes  = 9
	wTime = 9
	wTh   = 4
	wStat = 1
)

// HeaderLines returns the six lines above the process rows: four of stats, a
// rule, and the column headings.
func HeaderLines(s model.Summary, view model.ViewState, width int) []string {
	if width <= 0 {
		width = 80
	}
	load := "-"
	if s.HasLoad {
		load = fmt.Sprintf("%.2f, %.2f, %.2f", s.LoadAvg[0], s.LoadAvg[1], s.LoadAvg[2])
	}
	l0 := fmt.Sprintf("cctop - %s up %s,  load average: %s",
		s.Time.Format("15:04:05"), format.Uptime(int64(s.Uptime.Seconds())), load)
	l1 := fmt.Sprintf("Tasks: %d total, %d running, %d sleeping, %d stopped, %d zombie",
		s.TasksTotal, s.Running, s.Sleeping, s.Stopped, s.Zombie)
	l2 := fmt.Sprintf("%%CPU: %5s us, %5s sy, %5s id",
		format.Percent(s.CPUUser), format.Percent(s.CPUSys), format.Percent(s.CPUIdle))
	l3 := fmt.Sprintf("Mem : %9s total, %9s used, %9s free",
		format.Bytes(s.MemTotal, view.Unit), format.Bytes(s.MemUsed, view.Unit), format.Bytes(s.MemFree, view.Unit))
	sep := strings.Repeat("-", width)
	return []string{
		format.Truncate(l0, width),
		format.Truncate(l1, width),
		format.Truncate(l2, width),
		format.Truncate(l3, width),
		sep,
		columnHeader(width),
	}
}

func columnHeader(width int) string {
	h := strings.Join([]string{
		format.PadLeft("PID", wPID),
		format.PadRight("USER", wUser),
		format.PadLeft("%CPU", wCPU),
		format.PadLeft("%MEM", wMem),
		format.PadLeft("RES", wRes),
		format.PadLeft("TIME+", wTime),
		format.PadLeft("#TH", wTh),
		"S",
		"COMMAND",
	}, " ")
	return format.Truncate(h, width)
}

func ProcLine(p model.Process, unit model.MemUnit, width int) string {
	cmdWidth := width - fixedColumnsWidth()
	if cmdWidth < 1 {
		cmdWidth = 1
	}
	row := strings.Join([]string{
		format.PadLeft(fmt.Sprintf("%d", p.PID), wPID),
		format.PadRight(p.User, wUser),
		format.PadLeft(format.Percent(p.CPUPercent), wCPU),
		format.PadLeft(format.Percent(p.MemPercent), wMem),
		format.PadLeft(format.Bytes(p.RSS, unit), wRes),
		format.PadLeft(format.TimePlus(p.CPUTime), wTime),
		format.PadLeft(fmt.Sprintf("%d", p.Threads), wTh),
		stateLetter(p.State),
		format.Truncate(p.Command, cmdWidth),
	}, " ")
	return format.Truncate(row, width)
}

// fixedColumnsWidth is the width of every column before COMMAND, plus the
// space after each.
func fixedColumnsWidth() int {
	return wPID + wUser + wCPU + wMem + wRes + wTime + wTh + wStat + 8
}

func stateLetter(state string) string {
	if state == "" {
		return "S"
	}
	r := []rune(state)
	return strings.ToUpper(string(r[0]))
}

// StatusLine is the help and status bar at the bottom.
func StatusLine(view model.ViewState, delaySecs float64, width int) string {
	order := "desc"
	if view.Reverse {
		order = "asc"
	}
	pause := ""
	if view.Paused {
		pause = " [PAUSED]"
	}
	s := fmt.Sprintf(" Sort: %s(%s)  Unit: %s  Delay: %.1fs%s   [q]uit [s]ort [r]ev [e]unit [p]ause [space]refresh",
		view.Sort, order, view.Unit, delaySecs, pause)
	return format.Truncate(s, width)
}

// Frame renders snap as lines of text, fitted to width x height. A height of
// 0 or less shows every process.
func Frame(snap model.Snapshot, delaySecs float64, width, height int) []string {
	if width <= 0 {
		width = 80
	}
	lines := HeaderLines(snap.Summary, snap.View, width)

	bodyRows := len(snap.Procs)
	if height > 0 {
		// Leave the last line for the status bar.
		bodyRows = min(bodyRows, max(height-len(lines)-1, 0))
	}
	for i := 0; i < bodyRows; i++ {
		lines = append(lines, ProcLine(snap.Procs[i], snap.View.Unit, width))
	}
	lines = append(lines, StatusLine(snap.View, delaySecs, width))
	return lines
}
