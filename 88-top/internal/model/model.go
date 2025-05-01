// Package model has the types shared by the engine and the TUI.
package model

import "time"

type SortKey int

const (
	SortCPU SortKey = iota
	SortMem
	SortPID
	SortTime
	SortUser
	SortCommand
)

// sortCycle is the order the `s` key cycles through.
var sortCycle = []SortKey{SortCPU, SortMem, SortPID, SortTime, SortUser, SortCommand}

func (k SortKey) Next() SortKey {
	for i, c := range sortCycle {
		if c == k {
			return sortCycle[(i+1)%len(sortCycle)]
		}
	}
	return SortCPU
}

// String returns the column heading, e.g. "%CPU".
func (k SortKey) String() string {
	switch k {
	case SortCPU:
		return "%CPU"
	case SortMem:
		return "%MEM"
	case SortPID:
		return "PID"
	case SortTime:
		return "TIME+"
	case SortUser:
		return "USER"
	case SortCommand:
		return "COMMAND"
	default:
		return "?"
	}
}

// ParseSortKey parses the -s flag.
func ParseSortKey(s string) (SortKey, bool) {
	switch s {
	case "cpu":
		return SortCPU, true
	case "mem":
		return SortMem, true
	case "pid":
		return SortPID, true
	case "time":
		return SortTime, true
	case "user":
		return SortUser, true
	case "command", "cmd":
		return SortCommand, true
	default:
		return SortCPU, false
	}
}

type MemUnit int

const (
	UnitAuto MemUnit = iota
	UnitKiB
	UnitMiB
	UnitGiB
)

// Next cycles KiB, MiB, GiB, auto.
func (u MemUnit) Next() MemUnit {
	switch u {
	case UnitKiB:
		return UnitMiB
	case UnitMiB:
		return UnitGiB
	case UnitGiB:
		return UnitAuto
	default:
		return UnitKiB
	}
}

func (u MemUnit) String() string {
	switch u {
	case UnitKiB:
		return "KiB"
	case UnitMiB:
		return "MiB"
	case UnitGiB:
		return "GiB"
	default:
		return "auto"
	}
}

// ParseMemUnit parses the -M flag.
func ParseMemUnit(s string) (MemUnit, bool) {
	switch s {
	case "auto":
		return UnitAuto, true
	case "kib", "kb", "k":
		return UnitKiB, true
	case "mib", "mb", "m":
		return UnitMiB, true
	case "gib", "gb", "g":
		return UnitGiB, true
	default:
		return UnitAuto, false
	}
}

// ViewState is the display settings the user can change with keys.
type ViewState struct {
	Sort       SortKey
	Reverse    bool // flip the column's usual sort order
	Unit       MemUnit
	UserFilter string
	Paused     bool
	Solaris    bool // divide %CPU by the number of CPUs
}

// Process is one row of the process table.
type Process struct {
	PID        int
	PPID       int
	User       string
	Command    string
	State      string
	CPUPercent float64
	MemPercent float64
	RSS        uint64
	CPUTime    float64 // seconds, shown as TIME+
	Threads    int
}

// Summary is the header above the table.
type Summary struct {
	Time       time.Time
	Uptime     time.Duration
	LoadAvg    [3]float64
	HasLoad    bool
	NumCPU     int
	TasksTotal int
	Running    int
	Sleeping   int
	Stopped    int
	Zombie     int
	CPUUser    float64
	CPUSys     float64
	CPUIdle    float64
	MemTotal   uint64
	MemUsed    uint64
	MemFree    uint64
}

// Snapshot is everything needed to draw one frame.
type Snapshot struct {
	Summary Summary
	Procs   []Process
	View    ViewState
}
