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
	enableVirtualTerminalInput      = 0x0200
	enableProcessedOutput           = 0x0001
	enableVirtualTerminalProcessing = 0x0004
)

type state struct{ in, out uint32 }

// openTTY opens the console devices, which are separate from stdin/stdout and
// stay usable when the item list is being piped in.
func openTTY() (*os.File, *os.File, error) {
	in, err := os.OpenFile("CONIN$", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err
	}
	out, err := os.OpenFile("CONOUT$", os.O_RDWR, 0)
	if err != nil {
		in.Close()
		return nil, nil, err
	}
	return in, out, nil
}

func consoleMode(f *os.File) (uint32, error) {
	var mode uint32
	r, _, err := procGetConsoleMode.Call(f.Fd(), uintptr(unsafe.Pointer(&mode)))
	if r == 0 {
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

// makeRaw turns off line buffering and echo on the input side, and turns on VT
// processing on both sides so that the same escape sequences work as on Unix.
func makeRaw(in, out *os.File) (*state, error) {
	oldIn, err := consoleMode(in)
	if err != nil {
		return nil, err
	}
	oldOut, err := consoleMode(out)
	if err != nil {
		return nil, err
	}
	st := &state{in: oldIn, out: oldOut}
	newIn := oldIn&^(enableProcessedInput|enableLineInput|enableEchoInput) | enableVirtualTerminalInput
	if err := setConsoleMode(in, newIn); err != nil {
		return nil, err
	}
	newOut := oldOut | enableProcessedOutput | enableVirtualTerminalProcessing
	if err := setConsoleMode(out, newOut); err != nil {
		setConsoleMode(in, oldIn)
		return nil, err
	}
	return st, nil
}

func restore(in, out *os.File, st *state) {
	setConsoleMode(in, st.in)
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

func size(out *os.File) (int, int, error) {
	var info consoleScreenBufferInfo
	r, _, err := procGetConsoleScreenBufferInfo.Call(out.Fd(), uintptr(unsafe.Pointer(&info)))
	if r == 0 {
		return 0, 0, err
	}
	// The visible window, not the (usually taller) scrollback buffer.
	return int(info.Window.Right-info.Window.Left) + 1, int(info.Window.Bottom-info.Window.Top) + 1, nil
}
