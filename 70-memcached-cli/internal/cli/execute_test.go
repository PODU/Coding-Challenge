package cli

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/codingchallenges/ccmc/internal/client"
)

// scriptServer is a tiny listener that, per accepted connection, reads request
// lines and replies from a fixed script keyed by the command's first word. It
// is enough to exercise Execute's dispatch, output streams, and exit codes
// without a full memcached implementation.
type scriptServer struct {
	ln      net.Listener
	replies map[string]string // first request word -> raw reply bytes
	// storageData true means the command sends a data block we must drain.
}

func newScriptServer(t *testing.T, replies map[string]string) *scriptServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &scriptServer{ln: ln, replies: replies}
	go s.serve()
	t.Cleanup(func() { ln.Close() })
	return s
}

func (s *scriptServer) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *scriptServer) handle(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		trimmed := strings.TrimRight(line, "\r\n")
		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}
		cmd := fields[0]
		// Drain a data block for storage commands (bytes is the 5th field).
		switch cmd {
		case "set", "add", "replace", "append", "prepend", "cas":
			if n := atoiSafe(fields, 4); n >= 0 {
				buf := make([]byte, n+2)
				io.ReadFull(r, buf)
			}
		}
		if reply, ok := s.replies[cmd]; ok {
			io.WriteString(conn, reply)
		} else {
			io.WriteString(conn, "ERROR\r\n")
		}
	}
}

func atoiSafe(fields []string, i int) int {
	if i >= len(fields) {
		return -1
	}
	n, err := strconv.Atoi(fields[i])
	if err != nil {
		return -1
	}
	return n
}

func (s *scriptServer) dial(t *testing.T) *client.Client {
	t.Helper()
	host, port, _ := net.SplitHostPort(s.ln.Addr().String())
	c, err := client.Dial(host, port, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestExecuteSetStored(t *testing.T) {
	s := newScriptServer(t, map[string]string{"set": "STORED\r\n"})
	c := s.dial(t)

	cmd, _ := ParseCommand([]string{"set", "k", "v"}, Globals{}, nil)
	var out, errw bytes.Buffer
	code := Execute(c, cmd, &out, &errw)
	if code != ExitOK {
		t.Fatalf("exit = %d, stderr=%q", code, errw.String())
	}
	if strings.TrimSpace(out.String()) != "STORED" {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestExecuteGetHit(t *testing.T) {
	s := newScriptServer(t, map[string]string{"get": "VALUE k 0 5\r\nhello\r\nEND\r\n"})
	c := s.dial(t)

	cmd, _ := ParseCommand([]string{"get", "k"}, Globals{}, nil)
	var out, errw bytes.Buffer
	code := Execute(c, cmd, &out, &errw)
	if code != ExitOK {
		t.Fatalf("exit = %d", code)
	}
	if strings.TrimSpace(out.String()) != "k: hello" {
		t.Fatalf("stdout = %q", out.String())
	}
}

func TestExecuteGetMissExitCode(t *testing.T) {
	s := newScriptServer(t, map[string]string{"get": "END\r\n"})
	c := s.dial(t)

	cmd, _ := ParseCommand([]string{"get", "k"}, Globals{}, nil)
	var out, errw bytes.Buffer
	code := Execute(c, cmd, &out, &errw)
	if code != ExitMiss {
		t.Fatalf("exit = %d, want %d", code, ExitMiss)
	}
	if !strings.Contains(errw.String(), "(not found)") {
		t.Fatalf("stderr = %q", errw.String())
	}
}

func TestExecuteAddNotStoredExitCode(t *testing.T) {
	s := newScriptServer(t, map[string]string{"add": "NOT_STORED\r\n"})
	c := s.dial(t)

	cmd, _ := ParseCommand([]string{"add", "k", "v"}, Globals{}, nil)
	var out, errw bytes.Buffer
	code := Execute(c, cmd, &out, &errw)
	if code != ExitMiss {
		t.Fatalf("exit = %d, want %d (NOT_STORED is a logical miss)", code, ExitMiss)
	}
}

func TestExecuteProtocolErrorExitCode(t *testing.T) {
	s := newScriptServer(t, map[string]string{"get": "SERVER_ERROR out of memory\r\n"})
	c := s.dial(t)

	cmd, _ := ParseCommand([]string{"get", "k"}, Globals{}, nil)
	var out, errw bytes.Buffer
	code := Execute(c, cmd, &out, &errw)
	if code != ExitProtocol {
		t.Fatalf("exit = %d, want %d", code, ExitProtocol)
	}
	if !strings.Contains(errw.String(), "out of memory") {
		t.Fatalf("stderr = %q", errw.String())
	}
}
