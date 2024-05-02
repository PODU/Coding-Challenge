package main

import (
	"bytes"
	"net"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

// TestUDPDatagramRelay exercises udpServerRelay over loopback: a client sends a
// datagram, the server prints it and replies, and the client reads the reply.
func TestUDPDatagramRelay(t *testing.T) {
	saddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv, err := net.ListenUDP("udp", saddr)
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	var serverOut bytes.Buffer
	serverIn := bytes.NewReader([]byte("pong"))
	go udpServerRelay(srv, serverIn, &serverOut)

	cli, err := net.DialUDP("udp", nil, srv.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	if _, err := cli.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}

	cli.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 64)
	n, err := cli.Read(buf)
	if err != nil {
		t.Fatalf("client read: %v", err)
	}
	if got := string(buf[:n]); got != "pong" {
		t.Fatalf("client got %q, want pong", got)
	}

	// Give the server a moment to record the inbound datagram.
	deadline := time.Now().Add(2 * time.Second)
	for serverOut.Len() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := serverOut.String(); got != "ping" {
		t.Fatalf("server received %q, want ping", got)
	}
}

// TestScanPorts opens known listeners and leaves a gap, then asserts -z reports
// exactly the open ports in the range.
func TestScanPorts(t *testing.T) {
	// Two open listeners.
	l1, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l1.Close()
	l2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l2.Close()

	p1 := l1.Addr().(*net.TCPAddr).Port
	p2 := l2.Addr().(*net.TCPAddr).Port

	// A closed port: bind then immediately release it.
	l3, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	pClosed := l3.Addr().(*net.TCPAddr).Port
	l3.Close()

	lo, hi := min3(p1, p2, pClosed), max3(p1, p2, pClosed)
	open := scanPorts("tcp", "127.0.0.1", lo, hi, 500*time.Millisecond)

	set := map[int]bool{}
	for _, p := range open {
		set[p] = true
	}
	if !set[p1] || !set[p2] {
		t.Fatalf("expected open ports %d,%d in %v", p1, p2, open)
	}
	if set[pClosed] {
		t.Fatalf("closed port %d reported open in %v", pClosed, open)
	}
}

// TestRunExec wires a trivial child process's stdout to a conn and asserts the
// output is relayed back over the connection. Skipped if no shell is available.
func TestRunExec(t *testing.T) {
	var prog string
	var args []string
	if runtime.GOOS == "windows" {
		prog, args = "cmd", []string{"/c", "echo hello"}
	} else {
		prog, args = "sh", []string{"-c", "echo hello"}
	}
	if _, err := exec.LookPath(prog); err != nil {
		t.Skipf("%s not available: %v", prog, err)
	}

	client, server := loopbackPair(t)
	defer client.Close()

	go func() {
		command := prog
		for _, a := range args {
			command += " " + a
		}
		runExec(server, command)
		server.Close()
	}()

	client.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, 256)
	n, _ := client.Read(buf)
	if !bytes.Contains(buf[:n], []byte("hello")) {
		t.Fatalf("exec output %q does not contain hello", buf[:n])
	}
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

func max3(a, b, c int) int {
	m := a
	if b > m {
		m = b
	}
	if c > m {
		m = c
	}
	return m
}
