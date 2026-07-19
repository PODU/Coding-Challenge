//go:build linux

package pty

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

// From <asm-generic/ioctls.h>.
const (
	tiocsptlck = 0x40045431 // (un)lock the slave pty
	tiocgptn   = 0x80045430 // get the slave pty number
	tiocswinsz = 0x5414     // set window size
)

type winsize struct {
	rows, cols, xpix, ypix uint16
}

type unixPTY struct {
	master *os.File
	cmd    *exec.Cmd
}

// OpenShell starts shell (or $SHELL, or /bin/sh) on a new PTY of size w x h.
// The shell gets its own session with the PTY as its controlling terminal.
func OpenShell(shell string, w, h int) (PTY, error) {
	if shell == "" {
		shell = os.Getenv("SHELL")
	}
	if shell == "" {
		shell = "/bin/sh"
	}
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, fmt.Errorf("open ptmx: %w", err)
	}
	// The equivalent of unlockpt() and ptsname().
	var unlock int32
	if err := ioctl(master.Fd(), tiocsptlck, uintptr(unsafe.Pointer(&unlock))); err != nil {
		master.Close()
		return nil, fmt.Errorf("unlockpt: %w", err)
	}
	var ptn uint32
	if err := ioctl(master.Fd(), tiocgptn, uintptr(unsafe.Pointer(&ptn))); err != nil {
		master.Close()
		return nil, fmt.Errorf("ptsname: %w", err)
	}
	slavePath := fmt.Sprintf("/dev/pts/%d", ptn)
	slave, err := os.OpenFile(slavePath, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		master.Close()
		return nil, fmt.Errorf("open slave: %w", err)
	}

	cmd := exec.Command(shell)
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
		Ctty:    int(slave.Fd()),
	}
	if err := cmd.Start(); err != nil {
		master.Close()
		slave.Close()
		return nil, fmt.Errorf("start shell: %w", err)
	}
	// The child holds the slave now; the parent only needs the master.
	slave.Close()

	p := &unixPTY{master: master, cmd: cmd}
	p.Resize(w, h)
	return p, nil
}

func (p *unixPTY) Read(b []byte) (int, error)  { return p.master.Read(b) }
func (p *unixPTY) Write(b []byte) (int, error) { return p.master.Write(b) }

func (p *unixPTY) Resize(w, h int) error {
	ws := winsize{rows: uint16(h), cols: uint16(w)}
	return ioctl(p.master.Fd(), tiocswinsz, uintptr(unsafe.Pointer(&ws)))
}

func (p *unixPTY) Close() error {
	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Kill()
		p.cmd.Wait()
	}
	return p.master.Close()
}

func ioctl(fd, request, arg uintptr) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, request, arg)
	if errno != 0 {
		return errno
	}
	return nil
}
