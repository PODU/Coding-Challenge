//go:build linux

// Package client is the cctmux front end. It starts a server if none is
// running, then either sends a one-off request (list, kill) or puts the
// terminal in raw mode and relays keys and screen updates until detached.
package client

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
	"unsafe"

	"cctmux/internal/protocol"
)

type Options struct {
	SockPath string
	Verb     protocol.Type // NewSession, AttachSession, ListSessions, KillSession
	Target   string        // session name
}

func Run(opt Options) error {
	conn, err := dialOrStart(opt.SockPath)
	if err != nil {
		return err
	}
	defer conn.Close()

	switch opt.Verb {
	case protocol.ListSessions:
		protocol.WriteMessage(conn, protocol.ListSessions, nil)
		reply, err := protocol.ReadMessage(conn)
		if err != nil {
			return err
		}
		fmt.Print(reply.Str())
		return nil
	case protocol.KillSession:
		protocol.WriteMessage(conn, protocol.KillSession, []byte(opt.Target))
		reply, _ := protocol.ReadMessage(conn)
		if reply.Type == protocol.Error {
			return fmt.Errorf("%s", reply.Str())
		}
		return nil
	}

	protocol.WriteMessage(conn, opt.Verb, []byte(opt.Target))

	restore, err := makeRaw(os.Stdin.Fd())
	if err != nil {
		return err
	}
	defer restore()

	w, h := termSize(os.Stdout.Fd())
	protocol.WriteMessage(conn, protocol.Resize, protocol.EncodeResize(w, h))
	go watchResize(conn)

	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				protocol.WriteMessage(conn, protocol.Input, buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	for {
		msg, err := protocol.ReadMessage(conn)
		if err != nil {
			break
		}
		switch msg.Type {
		case protocol.Output:
			os.Stdout.Write(msg.Payload)
		case protocol.Error:
			restore()
			fmt.Fprintln(os.Stderr, msg.Str())
			return nil
		}
	}
	// Clear on the way out so the user's shell prompt is clean.
	os.Stdout.WriteString("\x1b[2J\x1b[H")
	return nil
}

// dialOrStart connects to the server, starting one (this binary with the
// hidden __server command) if nothing is listening.
func dialOrStart(sock string) (net.Conn, error) {
	if c, err := net.Dial("unix", sock); err == nil {
		return c, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(exe, "__server", "-S", sock)
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start server: %w", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if c, err := net.Dial("unix", sock); err == nil {
			return c, nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return nil, fmt.Errorf("server did not come up on %s", sock)
}

func watchResize(conn net.Conn) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	for range ch {
		w, h := termSize(os.Stdout.Fd())
		protocol.WriteMessage(conn, protocol.Resize, protocol.EncodeResize(w, h))
	}
}

// ioctl requests for termios and the window size.
const (
	tcgets     = 0x5401
	tcsets     = 0x5402
	tiocgwinsz = 0x5413
)

type winsize struct{ rows, cols, x, y uint16 }

func ioctl(fd, req, arg uintptr) error {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, arg)
	if e != 0 {
		return e
	}
	return nil
}

func termSize(fd uintptr) (int, int) {
	var ws winsize
	if err := ioctl(fd, tiocgwinsz, uintptr(unsafe.Pointer(&ws))); err != nil || ws.cols == 0 {
		return 80, 24
	}
	return int(ws.cols), int(ws.rows)
}

// makeRaw puts the terminal in raw mode on the alternate screen and returns a
// function that undoes both.
func makeRaw(fd uintptr) (func(), error) {
	var old syscall.Termios
	if err := ioctl(fd, tcgets, uintptr(unsafe.Pointer(&old))); err != nil {
		return nil, err
	}
	raw := old
	raw.Iflag &^= syscall.BRKINT | syscall.ICRNL | syscall.INPCK | syscall.ISTRIP | syscall.IXON
	raw.Oflag &^= syscall.OPOST
	raw.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.IEXTEN | syscall.ISIG
	raw.Cflag |= syscall.CS8
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0
	if err := ioctl(fd, tcsets, uintptr(unsafe.Pointer(&raw))); err != nil {
		return nil, err
	}
	os.Stdout.WriteString("\x1b[?1049h")
	return func() {
		os.Stdout.WriteString("\x1b[?1049l")
		ioctl(fd, tcsets, uintptr(unsafe.Pointer(&old)))
	}, nil
}
