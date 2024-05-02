package main

import (
	"fmt"
	"io"
	"net"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

const dialTimeout = 5 * time.Second

const scanTimeout = 2 * time.Second // per port

// closeWriter is the half-close that *net.TCPConn supports: the peer sees EOF
// but we can keep reading its reply.
type closeWriter interface {
	CloseWrite() error
}

// relay copies in to conn and conn to out at the same time, and returns when
// the remote side closes, as nc does. At the end of in, the connection is
// half-closed if halfClose is set. The in -> conn goroutine is not waited
// for: it may be stuck reading a terminal, and once the peer is gone there is
// no point.
func relay(conn net.Conn, in io.Reader, out io.Writer, halfClose bool) error {
	outDone := make(chan error, 1)

	go func() {
		_, err := io.Copy(out, conn)
		outDone <- err
	}()

	go func() {
		io.Copy(conn, in)
		if halfClose {
			if cw, ok := conn.(closeWriter); ok {
				cw.CloseWrite()
			}
		}
	}()

	err := <-outDone
	if err == io.EOF {
		return nil
	}
	return err
}

// wrapStreams adds the -x hex dumps of both directions, written to stderr.
func wrapStreams(cfg *Config, in io.Reader, out, stderr io.Writer) (io.Reader, io.Writer) {
	if !cfg.Hex {
		return in, out
	}
	in = io.TeeReader(in, newHexSink(stderr))
	out = io.MultiWriter(out, newHexSink(stderr))
	return in, out
}

func runConnect(cfg *Config, stdin io.Reader, stdout, stderr io.Writer) error {
	addr := net.JoinHostPort(cfg.Host, cfg.Port)
	vlog(cfg, stderr, "Connecting to %s (%s)", addr, cfg.network())

	conn, err := net.DialTimeout(cfg.network(), addr, dialTimeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	vlog(cfg, stderr, "Connected to %s", conn.RemoteAddr())

	if cfg.Exec != "" {
		return runExec(conn, cfg.Exec)
	}

	in, out := wrapStreams(cfg, stdin, stdout, stderr)
	return relay(conn, in, out, !cfg.UDP) // UDP has no half-close
}

// runListen accepts a single connection, relays it, and returns.
func runListen(cfg *Config, stdin io.Reader, stdout, stderr io.Writer) error {
	addr := net.JoinHostPort(cfg.Host, cfg.Port)

	if cfg.UDP {
		uaddr, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			return err
		}
		conn, err := net.ListenUDP("udp", uaddr)
		if err != nil {
			return err
		}
		defer conn.Close()
		vlog(cfg, stderr, "Listening on %s (udp)", conn.LocalAddr())
		in, out := wrapStreams(cfg, stdin, stdout, stderr)
		return udpServerRelay(conn, in, out)
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()
	vlog(cfg, stderr, "Listening on %s (tcp)", ln.Addr())

	conn, err := ln.Accept()
	if err != nil {
		return err
	}
	defer conn.Close()
	vlog(cfg, stderr, "Connection from %s", conn.RemoteAddr())

	if cfg.Exec != "" {
		return runExec(conn, cfg.Exec)
	}

	in, out := wrapStreams(cfg, stdin, stdout, stderr)
	return relay(conn, in, out, true)
}

// udpServerRelay is relay for a UDP listener. There is no connection, so
// whoever sends the first datagram becomes the peer we reply to.
func udpServerRelay(conn *net.UDPConn, in io.Reader, out io.Writer) error {
	buf := make([]byte, 65535)

	n, peer, err := conn.ReadFromUDP(buf)
	if err != nil {
		return err
	}
	if n > 0 {
		if _, err := out.Write(buf[:n]); err != nil {
			return err
		}
	}

	// Each read from in is sent as one datagram.
	go func() {
		b := make([]byte, 65535)
		for {
			m, rerr := in.Read(b)
			if m > 0 {
				conn.WriteToUDP(b[:m], peer)
			}
			if rerr != nil {
				return
			}
		}
	}()

	// No EOF in UDP, so this only stops on an error.
	for {
		m, _, rerr := conn.ReadFromUDP(buf)
		if m > 0 {
			if _, werr := out.Write(buf[:m]); werr != nil {
				return werr
			}
		}
		if rerr != nil {
			return rerr
		}
	}
}

// runExec runs command with its stdin, stdout and stderr all on conn. The
// command is split on whitespace; there is no shell quoting.
func runExec(conn net.Conn, command string) error {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return fmt.Errorf("empty -e command")
	}
	cmd := exec.Command(fields[0], fields[1:]...)
	cmd.Stdin = conn
	cmd.Stdout = conn
	cmd.Stderr = conn
	return cmd.Run()
}

func runScan(cfg *Config, stdout, stderr io.Writer) error {
	lo, hi, err := parsePortRange(cfg.Port)
	if err != nil {
		return err
	}
	open := scanPorts(cfg.network(), cfg.Host, lo, hi, scanTimeout)
	for _, p := range open {
		fmt.Fprintf(stdout, "Connection to %s %d port [%s] succeeded!\n", cfg.Host, p, cfg.network())
	}
	return nil
}

// scanPorts returns the open ports in lo..hi, sorted. A fixed number of
// workers keeps a big range from using up file descriptors.
func scanPorts(network, host string, lo, hi int, timeout time.Duration) []int {
	const workers = 128
	ports := make(chan int)
	var mu sync.Mutex
	var open []int
	var wg sync.WaitGroup

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range ports {
				addr := net.JoinHostPort(host, fmt.Sprintf("%d", p))
				conn, err := net.DialTimeout(network, addr, timeout)
				if err != nil {
					continue
				}
				// For UDP a successful Dial does not prove the port is open,
				// but it matches nc's best-effort behaviour. Send nothing.
				conn.Close()
				mu.Lock()
				open = append(open, p)
				mu.Unlock()
			}
		}()
	}

	for p := lo; p <= hi; p++ {
		ports <- p
	}
	close(ports)
	wg.Wait()

	sort.Ints(open)
	return open
}

func vlog(cfg *Config, stderr io.Writer, format string, args ...any) {
	if cfg.Verbose {
		fmt.Fprintf(stderr, "ccnc: "+format+"\n", args...)
	}
}
