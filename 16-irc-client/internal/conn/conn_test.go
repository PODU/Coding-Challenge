package conn

import (
	"bufio"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"ircclient/internal/client"
	"ircclient/internal/parser"
)

// TestSendFraming verifies CRLF framing and truncation to the protocol limit.
func TestSendFraming(t *testing.T) {
	srv, cli := net.Pipe()
	defer srv.Close()
	c := New(cli)
	defer c.Close()

	got := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(srv).ReadString('\n')
		got <- line
	}()

	if err := c.Send("NICK CCClient"); err != nil {
		t.Fatal(err)
	}
	select {
	case line := <-got:
		if line != "NICK CCClient\r\n" {
			t.Errorf("framed line = %q, want %q", line, "NICK CCClient\r\n")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for framed line")
	}
}

func TestSendTruncatesOversize(t *testing.T) {
	srv, cli := net.Pipe()
	defer srv.Close()
	c := New(cli)
	defer c.Close()

	got := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(srv).ReadString('\n')
		got <- line
	}()

	long := "PRIVMSG #x :" + strings.Repeat("A", 1000)
	_ = c.Send(long)
	select {
	case line := <-got:
		if len(line) != maxLine {
			t.Errorf("oversize line length = %d, want %d (incl CRLF)", len(line), maxLine)
		}
		if !strings.HasSuffix(line, "\r\n") {
			t.Errorf("truncated line lost CRLF: %q", line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}
}

func TestReaderSplitsCoalescedLines(t *testing.T) {
	srv, cli := net.Pipe()
	defer cli.Close()
	c := New(cli)
	defer c.Close()

	go func() {
		// Two messages in one write, plus a split third.
		srv.Write([]byte("PING :a\r\n:srv NOTICE * :hi\r\nPRI"))
		srv.Write([]byte("VMSG #x :y\r\n"))
		srv.Close()
	}()

	var lines []string
	for l := range c.Lines() {
		lines = append(lines, l)
	}
	want := []string{"PING :a", ":srv NOTICE * :hi", "PRIVMSG #x :y"}
	if len(lines) != len(want) {
		t.Fatalf("got %v, want %v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line[%d] = %q, want %q", i, lines[i], want[i])
		}
	}
}

// TestWriteSerialisation fires many concurrent Sends and asserts no line is
// interleaved/garbled on the wire.
func TestWriteSerialisation(t *testing.T) {
	srv, cli := net.Pipe()
	c := New(cli)
	defer c.Close()

	const n = 100
	done := make(chan struct{})
	seen := make(map[string]bool)
	go func() {
		r := bufio.NewReader(srv)
		for i := 0; i < n; i++ {
			line, err := r.ReadString('\n')
			if err != nil {
				break
			}
			seen[strings.TrimRight(line, "\r\n")] = true
		}
		close(done)
	}()

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c.Send(parser.Encode("PRIVMSG", "#x", strings.Repeat("z", 20)))
		}(i)
	}
	wg.Wait()
	<-done
	for s := range seen {
		if s != "PRIVMSG #x :"+strings.Repeat("z", 20) {
			t.Fatalf("garbled line on wire: %q", s)
		}
	}
}

// TestSessionAgainstMockServer is the end-to-end integration test from the
// design's §7.2: a scripted in-process server drives registration, PING/PONG,
// a PRIVMSG, and a server ERROR, wired through the real conn + client.
func TestSessionAgainstMockServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	type result struct {
		gotNick string
		gotUser string
		gotPong string
		err     error
	}
	resCh := make(chan result, 1)

	go func() {
		var res result
		sc, err := ln.Accept()
		if err != nil {
			res.err = err
			resCh <- res
			return
		}
		defer sc.Close()
		r := bufio.NewReader(sc)
		write := func(s string) { sc.Write([]byte(s + "\r\n")) }

		// Expect NICK then USER.
		for i := 0; i < 2; i++ {
			line, err := r.ReadString('\n')
			if err != nil {
				res.err = err
				resCh <- res
				return
			}
			m, _ := parser.Parse(line)
			switch m.Command {
			case "NICK":
				res.gotNick = m.Target()
			case "USER":
				res.gotUser = m.Target()
			}
		}
		// Welcome, then PING.
		write(":mock.server 001 CCClient :Welcome CCClient")
		write("PING :probe-token")
		// Expect the matching PONG.
		line, err := r.ReadString('\n')
		if err != nil {
			res.err = err
			resCh <- res
			return
		}
		m, _ := parser.Parse(line)
		if m.Command == "PONG" {
			res.gotPong = m.Trailing()
		}
		// Push a channel message and then close the link.
		write(":Bob!~b@h PRIVMSG CCClient :ping from bob")
		write("ERROR :Closing Link: bye")
		resCh <- res
	}()

	c, err := Dial(t.Context(), ln.Addr().String(), false)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	cl := client.New(client.Config{Nick: "CCClient", User: "guest", Real: "Coding Challenges Client"}, c)
	if err := cl.Register(); err != nil {
		t.Fatal(err)
	}

	var events []client.Event
	shutdown := false
	timeout := time.After(5 * time.Second)
	for !shutdown {
		select {
		case line, ok := <-c.Lines():
			if !ok {
				shutdown = true
				break
			}
			m, perr := parser.Parse(line)
			if perr != nil {
				continue
			}
			for _, e := range cl.Handle(m) {
				if e.Kind == client.KindShutdown {
					shutdown = true
				}
				events = append(events, e)
			}
		case <-timeout:
			t.Fatal("integration test timed out")
		}
	}

	res := <-resCh
	if res.err != nil {
		t.Fatalf("server error: %v", res.err)
	}
	if res.gotNick != "CCClient" {
		t.Errorf("server saw NICK %q, want CCClient", res.gotNick)
	}
	if res.gotUser != "guest" {
		t.Errorf("server saw USER %q, want guest", res.gotUser)
	}
	if res.gotPong != "probe-token" {
		t.Errorf("server saw PONG token %q, want probe-token", res.gotPong)
	}
	if !cl.Registered() {
		t.Error("client never marked registered")
	}

	// The PRIVMSG (to our nick) must have surfaced as a PM event.
	foundPM := false
	for _, e := range events {
		if e.Kind == client.KindPM && strings.Contains(e.Text, "ping from bob") {
			foundPM = true
		}
	}
	if !foundPM {
		t.Errorf("PRIVMSG did not surface as PM event; events = %v", events)
	}
}
