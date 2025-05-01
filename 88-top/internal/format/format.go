// Package format formats sizes, times and percentages the way top does, and
// fits text into columns.
package format

import (
	"fmt"
	"math"
	"strings"

	"github.com/example/cctop/internal/model"
)

const (
	kib = 1024.0
	mib = 1024.0 * 1024.0
	gib = 1024.0 * 1024.0 * 1024.0
	tib = 1024.0 * 1024.0 * 1024.0 * 1024.0
)

// Bytes formats b in unit. UnitAuto picks the largest unit that keeps the
// number under 1024.
func Bytes(b uint64, unit model.MemUnit) string {
	f := float64(b)
	switch unit {
	case model.UnitKiB:
		return fmt.Sprintf("%.1fKi", f/kib)
	case model.UnitMiB:
		return fmt.Sprintf("%.1fMi", f/mib)
	case model.UnitGiB:
		return fmt.Sprintf("%.1fGi", f/gib)
	default:
		return autoBytes(f)
	}
}

func autoBytes(f float64) string {
	switch {
	case f >= tib:
		return fmt.Sprintf("%.1fTi", f/tib)
	case f >= gib:
		return fmt.Sprintf("%.1fGi", f/gib)
	case f >= mib:
		return fmt.Sprintf("%.1fMi", f/mib)
	case f >= kib:
		return fmt.Sprintf("%.1fKi", f/kib)
	default:
		return fmt.Sprintf("%.0fB", f)
	}
}

// TimePlus formats CPU seconds like top's TIME+ column: M:SS.CC under an
// hour (1:02.44), H:MM:SS after that (3:04:05).
func TimePlus(seconds float64) string {
	if seconds < 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		seconds = 0
	}
	totalCentis := int64(math.Round(seconds * 100))
	centis := totalCentis % 100
	totalSecs := totalCentis / 100
	secs := totalSecs % 60
	totalMins := totalSecs / 60
	mins := totalMins % 60
	hours := totalMins / 60
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, mins, secs)
	}
	return fmt.Sprintf("%d:%02d.%02d", mins, secs, centis)
}

// Uptime formats an uptime like top: "3:21" or "2 days, 4:05".
func Uptime(secs int64) string {
	if secs < 0 {
		secs = 0
	}
	days := secs / 86400
	rem := secs % 86400
	hours := rem / 3600
	mins := (rem % 3600) / 60
	switch {
	case days == 1:
		return fmt.Sprintf("1 day, %d:%02d", hours, mins)
	case days > 1:
		return fmt.Sprintf("%d days, %d:%02d", days, hours, mins)
	default:
		return fmt.Sprintf("%d:%02d", hours, mins)
	}
}

// Percent formats p with one decimal place, e.g. "6.3".
func Percent(p float64) string {
	if math.IsNaN(p) || math.IsInf(p, 0) {
		p = 0
	}
	return fmt.Sprintf("%.1f", p)
}

// Truncate cuts s to width runes, ending with "…" if anything was cut.
func Truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}

// PadRight left-aligns s in a column of the given width.
func PadRight(s string, width int) string {
	s = Truncate(s, width)
	if n := width - len([]rune(s)); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

// PadLeft right-aligns s in a column of the given width.
func PadLeft(s string, width int) string {
	s = Truncate(s, width)
	if n := width - len([]rune(s)); n > 0 {
		return strings.Repeat(" ", n) + s
	}
	return s
}
