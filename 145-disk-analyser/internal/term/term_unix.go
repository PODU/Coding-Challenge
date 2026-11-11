//go:build !windows

// Package term puts the terminal in raw mode and draws to it with plain
// ANSI escape codes: no curses, no terminal library.
package term

import (
	"os"
	"os/signal"
	"syscall"
	"unsafe"
)

type State struct{ termios syscall.Termios }

func ioctl(fd, req uintptr, arg unsafe.Pointer) error {
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg)); errno != 0 {
		return errno
	}
	return nil
}

// makeRaw is cfmakeraw: keys arrive one byte at a time with no echo, and
// Ctrl+C arrives as byte 3 rather than a signal. Output post-processing is
// left alone, which is harmless since every move is an absolute cursor jump.
func MakeRaw(in, out *os.File) (*State, error) {
	if _, _, err := Size(out); err != nil {
		return nil, err
	}
	var t syscall.Termios
	if err := ioctl(in.Fd(), ioctlGetTermios, unsafe.Pointer(&t)); err != nil {
		return nil, err
	}
	old := t
	t.Iflag &^= syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK | syscall.ISTRIP |
		syscall.INLCR | syscall.IGNCR | syscall.ICRNL | syscall.IXON
	t.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	t.Cflag &^= syscall.CSIZE | syscall.PARENB
	t.Cflag |= syscall.CS8
	t.Cc[syscall.VMIN] = 1
	t.Cc[syscall.VTIME] = 0
	if err := ioctl(in.Fd(), ioctlSetTermios, unsafe.Pointer(&t)); err != nil {
		return nil, err
	}
	return &State{termios: old}, nil
}

func Restore(in, out *os.File, st *State) {
	t := st.termios
	ioctl(in.Fd(), ioctlSetTermios, unsafe.Pointer(&t))
}

type winsize struct{ Row, Col, Xpixel, Ypixel uint16 }

func Size(out *os.File) (int, int, error) {
	var ws winsize
	if err := ioctl(out.Fd(), syscall.TIOCGWINSZ, unsafe.Pointer(&ws)); err != nil {
		return 0, 0, err
	}
	return int(ws.Col), int(ws.Row), nil
}

// resizeNotify turns SIGWINCH into a wake-up so a resize is redrawn at once
// rather than on the next frame.
func ResizeNotify() <-chan struct{} {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGWINCH)
	out := make(chan struct{}, 1)
	go func() {
		for range sig {
			select {
			case out <- struct{}{}:
			default:
			}
		}
	}()
	return out
}
