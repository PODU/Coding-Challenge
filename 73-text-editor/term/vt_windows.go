//go:build windows

package term

import (
	"os"

	"golang.org/x/sys/windows"
)

// enableVirtualTerminal makes the Windows console interpret ANSI escapes. The
// returned function restores the old console mode.
func enableVirtualTerminal(out *os.File) func() {
	handle := windows.Handle(out.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return func() {}
	}
	newMode := mode | windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
	if err := windows.SetConsoleMode(handle, newMode); err != nil {
		return func() {}
	}
	return func() {
		_ = windows.SetConsoleMode(handle, mode)
	}
}
