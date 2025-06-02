package smtp

import (
	"bufio"
	"bytes"
	"net"
	"strconv"
	"sync"
	"time"
)

// fakeConn is an in-memory net.Conn for driving a Session in unit tests. Reads
// come from an input buffer the test preloads; writes are captured in out.
type fakeConn struct {
	in  *bytes.Buffer
	out *bytes.Buffer
	mu  sync.Mutex
}

func newFakeConn(input string) *fakeConn {
	return &fakeConn{in: bytes.NewBufferString(input), out: &bytes.Buffer{}}
}

func (c *fakeConn) Read(p []byte) (int, error) {
	n, err := c.in.Read(p)
	return n, err
}

func (c *fakeConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.out.Write(p)
}

func (c *fakeConn) output() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.out.String()
}

func (c *fakeConn) Close() error                       { return nil }
func (c *fakeConn) LocalAddr() net.Addr                { return dummyAddr{} }
func (c *fakeConn) RemoteAddr() net.Addr               { return dummyAddr{} }
func (c *fakeConn) SetDeadline(t time.Time) error      { return nil }
func (c *fakeConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *fakeConn) SetWriteDeadline(t time.Time) error { return nil }

type dummyAddr struct{}

func (dummyAddr) Network() string { return "fake" }
func (dummyAddr) String() string  { return "fake" }

// newTestSession builds a Session over a fakeConn backed by a MemoryStore. The
// given input string (used by DATA reads) is preloaded into the read buffer.
func newTestSession(dataInput string) (*Session, *fakeConn, *MemoryStore) {
	store := NewMemoryStore()
	srv := NewServer("test.local", store, 0)
	conn := newFakeConn(dataInput)
	s := &Session{
		server: srv,
		conn:   conn,
		r:      bufio.NewReader(conn),
		w:      bufio.NewWriter(conn),
		state:  StateGreet,
	}
	return s, conn, store
}

// lastCode parses the SMTP reply code from the final reply line in out.
func lastCode(out string) int {
	lines := splitReplyLines(out)
	if len(lines) == 0 {
		return 0
	}
	last := lines[len(lines)-1]
	if len(last) < 3 {
		return 0
	}
	code, err := strconv.Atoi(last[:3])
	if err != nil {
		return 0
	}
	return code
}

func splitReplyLines(out string) []string {
	var lines []string
	for _, l := range bytes.Split([]byte(out), []byte("\r\n")) {
		if len(l) > 0 {
			lines = append(lines, string(l))
		}
	}
	return lines
}
