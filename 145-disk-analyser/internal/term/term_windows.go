//go:build windows

package term

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32                       = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode             = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode             = kernel32.NewProc("SetConsoleMode")
	procGetConsoleScreenBufferInfo = kernel32.NewProc("GetConsoleScreenBufferInfo")
)

const (
	enableProcessedInput            = 0x0001
	enableLineInput                 = 0x0002
	enableEchoInput                 = 0x0004
	enableQuickEditMode             = 0x0040
	enableExtendedFlags             = 0x0080
	enableVirtualTerminalInput      = 0x0200
	enableProcessedOutput           = 0x0001
	enableVirtualTerminalProcessing = 0x0004
)

type State struct{ in, out uint32 }

func consoleMode(f *os.File) (uint32, error) {
	var mode uint32
	if r, _, err := procGetConsoleMode.Call(f.Fd(), uintptr(unsafe.Pointer(&mode))); r == 0 {
		return 0, err
	}
	return mode, nil
}

func setConsoleMode(f *os.File, mode uint32) error {
	if r, _, err := procSetConsoleMode.Call(f.Fd(), uintptr(mode)); r == 0 {
		return err
	}
	return nil
}

// makeRaw turns off line input and echo, and turns on VT sequences in both
// directions so the same escape codes work as on Unix. Quick-edit is turned
// off so mouse clicks reach the program instead of selecting text.
func MakeRaw(in, out *os.File) (*State, error) {
	oldIn, err := consoleMode(in)
	if err != nil {
		return nil, err
	}
	oldOut, err := consoleMode(out)
	if err != nil {
		return nil, err
	}
	newIn := oldIn&^(enableProcessedInput|enableLineInput|enableEchoInput|enableQuickEditMode) |
		enableVirtualTerminalInput | enableExtendedFlags
	if err := setConsoleMode(in, newIn); err != nil {
		return nil, err
	}
	if err := setConsoleMode(out, oldOut|enableProcessedOutput|enableVirtualTerminalProcessing); err != nil {
		setConsoleMode(in, oldIn)
		return nil, err
	}
	return &State{in: oldIn, out: oldOut}, nil
}

func Restore(in, out *os.File, st *State) {
	// Extended flags must be set for the quick-edit bit to be applied.
	setConsoleMode(in, st.in|enableExtendedFlags)
	setConsoleMode(out, st.out)
}

type coord struct{ X, Y int16 }
type smallRect struct{ Left, Top, Right, Bottom int16 }

type consoleScreenBufferInfo struct {
	Size              coord
	CursorPosition    coord
	Attributes        uint16
	Window            smallRect
	MaximumWindowSize coord
}

func Size(out *os.File) (int, int, error) {
	var info consoleScreenBufferInfo
	if r, _, err := procGetConsoleScreenBufferInfo.Call(out.Fd(), uintptr(unsafe.Pointer(&info))); r == 0 {
		return 0, 0, err
	}
	// The visible window, not the (usually taller) scrollback buffer.
	return int(info.Window.Right-info.Window.Left) + 1, int(info.Window.Bottom-info.Window.Top) + 1, nil
}

// Windows has no SIGWINCH; callers poll Size instead.
func ResizeNotify() <-chan struct{} { return nil }
