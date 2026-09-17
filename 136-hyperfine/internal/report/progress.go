package report

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// barWidth is the number of cells in the drawn bar.
const barWidth = 30

// Bar is a single-line progress indicator redrawn in place on stderr. It is
// only useful on a terminal, so callers construct it with a Style whose
// Progress flag has already been resolved.
type Bar struct {
	Out     io.Writer
	Style   Style
	Enabled bool

	total   int
	started time.Time
	drawn   int // printed width of the last line, so we can erase it exactly
}

// NewBar returns a Bar that draws to out, or an inert Bar if the style says
// progress should be suppressed.
func NewBar(out io.Writer, s Style) *Bar {
	return &Bar{Out: out, Style: s, Enabled: s.Progress}
}

// Start begins (or restarts) a bar for total steps. Restarting matters because
// the run count is only known after the first timed execution.
func (b *Bar) Start(total int) {
	if !b.Enabled {
		return
	}
	if b.started.IsZero() {
		b.started = time.Now()
	}
	b.total = total
	b.render(0, total, 0)
}

// Advance redraws the bar after done steps, using estimate (in seconds) as the
// projected total runtime.
func (b *Bar) Advance(done, total int, estimate float64) {
	if !b.Enabled {
		return
	}
	b.total = total
	b.render(done, total, estimate)
}

// Finish erases the bar so the statistics block starts on a clean line.
func (b *Bar) Finish() {
	if !b.Enabled || b.drawn == 0 {
		return
	}
	fmt.Fprintf(b.Out, "\r%s\r", strings.Repeat(" ", b.drawn))
	b.drawn = 0
	b.started = time.Time{}
}

func (b *Bar) render(done, total int, estimate float64) {
	if total <= 0 {
		total = 1
	}
	frac := float64(done) / float64(total)
	if frac > 1 {
		frac = 1
	}
	filled := int(frac * barWidth)

	full, empty := "█", "░"
	if !b.Style.Unicode {
		full, empty = "#", "-"
	}
	bar := strings.Repeat(full, filled) + strings.Repeat(empty, barWidth-filled)

	eta := ""
	if done > 0 && estimate > 0 {
		remaining := estimate - time.Since(b.started).Seconds()
		if remaining < 0 {
			remaining = 0
		}
		eta = fmt.Sprintf(" ETA %s", shortDuration(remaining))
	}

	line := fmt.Sprintf("  [%s] %d/%d%s", bar, done, total, eta)
	pad := ""
	if n := len([]rune(line)); n < b.drawn {
		pad = strings.Repeat(" ", b.drawn-n)
	}
	fmt.Fprintf(b.Out, "\r%s%s", b.Style.cyan(line), pad)
	b.drawn = len([]rune(line))
}

// shortDuration renders a remaining time compactly: sub-minute values read
// better in seconds than as a zero-padded clock.
func shortDuration(seconds float64) string {
	switch {
	case seconds < 60:
		return fmt.Sprintf("%.1fs", seconds)
	case seconds < 3600:
		return fmt.Sprintf("%dm%02ds", int(seconds)/60, int(seconds)%60)
	default:
		return fmt.Sprintf("%dh%02dm", int(seconds)/3600, (int(seconds)%3600)/60)
	}
}
