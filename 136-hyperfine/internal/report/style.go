package report

import (
	"os"
)

// Style captures the three independent presentation decisions `--style` folds
// into one flag: colour, unicode, and whether a progress bar is drawn.
type Style struct {
	Color    bool
	Unicode  bool
	Progress bool
}

// StyleFor resolves a `--style` value against the terminal we are attached to.
// "auto" enables everything only when stderr is a terminal, so piping output
// into a file or another program yields clean, parseable text.
func StyleFor(name string, stderr *os.File) Style {
	tty := IsTerminal(stderr)
	switch name {
	case "basic":
		return Style{Color: false, Unicode: false, Progress: false}
	case "full":
		return Style{Color: true, Unicode: true, Progress: true}
	case "color":
		return Style{Color: true, Unicode: tty, Progress: tty}
	case "nocolor":
		return Style{Color: false, Unicode: tty, Progress: tty}
	case "none":
		return Style{Color: false, Unicode: false, Progress: false}
	default: // auto
		if os.Getenv("NO_COLOR") != "" {
			return Style{Color: false, Unicode: tty, Progress: tty}
		}
		return Style{Color: tty, Unicode: tty, Progress: tty}
	}
}

// IsTerminal reports whether f is attached to a character device. This is the
// portable approximation of isatty: good enough to decide whether escape codes
// and carriage-return redraws will look right.
func IsTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// ANSI attributes used by the reporter.
const (
	reset   = "\x1b[0m"
	bold    = "\x1b[1m"
	red     = "\x1b[31m"
	green   = "\x1b[32m"
	yellow  = "\x1b[33m"
	blue    = "\x1b[34m"
	magenta = "\x1b[35m"
	cyan    = "\x1b[36m"
)

func (s Style) paint(code, text string) string {
	if !s.Color {
		return text
	}
	return code + text + reset
}

func (s Style) bold(t string) string    { return s.paint(bold, t) }
func (s Style) green(t string) string   { return s.paint(green, t) }
func (s Style) greenB(t string) string  { return s.paint(bold+green, t) }
func (s Style) blue(t string) string    { return s.paint(blue, t) }
func (s Style) cyan(t string) string    { return s.paint(cyan, t) }
func (s Style) magenta(t string) string { return s.paint(magenta, t) }
func (s Style) yellow(t string) string  { return s.paint(yellow, t) }

// sym returns the unicode glyph when the terminal can render it, and an ASCII
// stand-in otherwise.
func (s Style) sym(unicode, ascii string) string {
	if s.Unicode {
		return unicode
	}
	return ascii
}
