//go:build !windows

package term

import (
	"os"
	"syscall"
	"unsafe"
)

type state struct{ termios syscall.Termios }

// openTTY opens the controlling terminal. Standard input cannot be used: it is
// where the item list arrives from.
func openTTY() (*os.File, *os.File, error) {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err
	}
	return f, f, nil
}

func ioctl(fd, req uintptr, arg unsafe.Pointer) error {
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg)); errno != 0 {
		return errno
	}
	return nil
}

// makeRaw is cfmakeraw: no line buffering, no echo, and no signal generation,
// so that Ctrl-C arrives as an ordinary key the program can act on.
func makeRaw(in, out *os.File) (*state, error) {
	var t syscall.Termios
	if err := ioctl(in.Fd(), ioctlGetTermios, unsafe.Pointer(&t)); err != nil {
		return nil, err
	}
	old := t
	t.Iflag &^= syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK | syscall.ISTRIP |
		syscall.INLCR | syscall.IGNCR | syscall.ICRNL | syscall.IXON
	t.Oflag &^= syscall.OPOST
	t.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	t.Cflag &^= syscall.CSIZE | syscall.PARENB
	t.Cflag |= syscall.CS8
	t.Cc[syscall.VMIN] = 1
	t.Cc[syscall.VTIME] = 0
	if err := ioctl(in.Fd(), ioctlSetTermios, unsafe.Pointer(&t)); err != nil {
		return nil, err
	}
	return &state{termios: old}, nil
}

func restore(in, out *os.File, st *state) {
	t := st.termios
	ioctl(in.Fd(), ioctlSetTermios, unsafe.Pointer(&t))
}

type winsize struct {
	Row, Col, Xpixel, Ypixel uint16
}

func size(out *os.File) (int, int, error) {
	var ws winsize
	if err := ioctl(out.Fd(), syscall.TIOCGWINSZ, unsafe.Pointer(&ws)); err != nil {
		return 0, 0, err
	}
	return int(ws.Col), int(ws.Row), nil
}
