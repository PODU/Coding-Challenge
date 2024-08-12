package client

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// fakeServer is a minimal in-memory memcached text-protocol server used by the
// integration tests. It supports set/add/replace/append/prepend/get/gets/
// delete/incr/decr/stats with correct byte-count framing.
type fakeServer struct {
	ln    net.Listener
	mu    sync.Mutex
	items map[string]fakeItem
	casID uint64
	wg    sync.WaitGroup
}

type fakeItem struct {
	flags uint32
	data  []byte
	cas   uint64
}

// newFakeServer starts a server on 127.0.0.1:0 and returns it with its address.
func newFakeServer(t *testing.T) *fakeServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	s := &fakeServer{ln: ln, items: make(map[string]fakeItem)}
	s.wg.Add(1)
	go s.serve()
	t.Cleanup(func() { s.Close() })
	return s
}

func (s *fakeServer) addr() string { return s.ln.Addr().String() }

func (s *fakeServer) hostPort() (string, string) {
	host, port, _ := net.SplitHostPort(s.addr())
	return host, port
}

func (s *fakeServer) Close() {
	s.ln.Close()
	s.wg.Wait()
}

func (s *fakeServer) serve() {
	defer s.wg.Done()
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handle(conn)
		}()
	}
}

func (s *fakeServer) handle(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "set", "add", "replace", "append", "prepend", "cas":
			s.handleStorage(r, w, fields)
		case "get", "gets":
			s.handleGet(w, fields)
		case "delete":
			s.handleDelete(w, fields)
		case "incr", "decr":
			s.handleIncrDecr(w, fields)
		case "stats":
			s.handleStats(w)
		case "quit":
			return
		default:
			io.WriteString(w, "ERROR\r\n")
		}
		w.Flush()
	}
}

func (s *fakeServer) handleStorage(r *bufio.Reader, w *bufio.Writer, f []string) {
	// <cmd> <key> <flags> <exptime> <bytes> [<cas>] [noreply]
	cmd := f[0]
	noreply := len(f) > 0 && f[len(f)-1] == "noreply"
	min := 5
	if cmd == "cas" {
		min = 6
	}
	if len(f) < min {
		io.WriteString(w, "ERROR\r\n")
		return
	}
	key := f[1]
	flags64, _ := strconv.ParseUint(f[2], 10, 32)
	nbytes, err := strconv.Atoi(f[4])
	if err != nil || nbytes < 0 {
		io.WriteString(w, "CLIENT_ERROR bad command line format\r\n")
		return
	}
	var casUnique uint64
	if cmd == "cas" {
		casUnique, _ = strconv.ParseUint(f[5], 10, 64)
	}

	data := make([]byte, nbytes)
	if _, err := io.ReadFull(r, data); err != nil {
		return
	}
	crlf := make([]byte, 2)
	io.ReadFull(r, crlf)
	if crlf[0] != '\r' || crlf[1] != '\n' {
		if !noreply {
			io.WriteString(w, "CLIENT_ERROR bad data chunk\r\n")
		}
		return
	}

	reply := s.applyStorage(cmd, key, uint32(flags64), data, casUnique)
	if !noreply {
		io.WriteString(w, reply+"\r\n")
	}
}

func (s *fakeServer) applyStorage(cmd, key string, flags uint32, data []byte, casUnique uint64) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.items[key]
	switch cmd {
	case "set":
		s.store(key, flags, data)
		return "STORED"
	case "add":
		if ok {
			return "NOT_STORED"
		}
		s.store(key, flags, data)
		return "STORED"
	case "replace":
		if !ok {
			return "NOT_STORED"
		}
		s.store(key, flags, data)
		return "STORED"
	case "append":
		if !ok {
			return "NOT_STORED"
		}
		s.store(key, existing.flags, append(append([]byte(nil), existing.data...), data...))
		return "STORED"
	case "prepend":
		if !ok {
			return "NOT_STORED"
		}
		s.store(key, existing.flags, append(append([]byte(nil), data...), existing.data...))
		return "STORED"
	case "cas":
		if !ok {
			return "NOT_FOUND"
		}
		if existing.cas != casUnique {
			return "EXISTS"
		}
		s.store(key, flags, data)
		return "STORED"
	}
	return "ERROR"
}

// store assigns a fresh cas id; caller holds the lock.
func (s *fakeServer) store(key string, flags uint32, data []byte) {
	s.casID++
	s.items[key] = fakeItem{flags: flags, data: data, cas: s.casID}
}

func (s *fakeServer) handleGet(w *bufio.Writer, f []string) {
	withCAS := f[0] == "gets"
	s.mu.Lock()
	for _, key := range f[1:] {
		item, ok := s.items[key]
		if !ok {
			continue
		}
		if withCAS {
			fmt.Fprintf(w, "VALUE %s %d %d %d\r\n", key, item.flags, len(item.data), item.cas)
		} else {
			fmt.Fprintf(w, "VALUE %s %d %d\r\n", key, item.flags, len(item.data))
		}
		w.Write(item.data)
		io.WriteString(w, "\r\n")
	}
	s.mu.Unlock()
	io.WriteString(w, "END\r\n")
}

func (s *fakeServer) handleDelete(w *bufio.Writer, f []string) {
	noreply := f[len(f)-1] == "noreply"
	if len(f) < 2 {
		io.WriteString(w, "ERROR\r\n")
		return
	}
	key := f[1]
	s.mu.Lock()
	_, ok := s.items[key]
	if ok {
		delete(s.items, key)
	}
	s.mu.Unlock()
	if noreply {
		return
	}
	if ok {
		io.WriteString(w, "DELETED\r\n")
	} else {
		io.WriteString(w, "NOT_FOUND\r\n")
	}
}

func (s *fakeServer) handleIncrDecr(w *bufio.Writer, f []string) {
	if len(f) < 3 {
		io.WriteString(w, "ERROR\r\n")
		return
	}
	key := f[1]
	delta, err := strconv.ParseUint(f[2], 10, 64)
	if err != nil {
		io.WriteString(w, "CLIENT_ERROR invalid numeric delta argument\r\n")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[key]
	if !ok {
		io.WriteString(w, "NOT_FOUND\r\n")
		return
	}
	cur, perr := strconv.ParseUint(strings.TrimSpace(string(item.data)), 10, 64)
	if perr != nil {
		io.WriteString(w, "CLIENT_ERROR cannot increment or decrement non-numeric value\r\n")
		return
	}
	var next uint64
	if f[0] == "incr" {
		next = cur + delta
	} else if delta > cur {
		next = 0
	} else {
		next = cur - delta
	}
	item.data = []byte(strconv.FormatUint(next, 10))
	s.casID++
	item.cas = s.casID
	s.items[key] = item
	fmt.Fprintf(w, "%d\r\n", next)
}

func (s *fakeServer) handleStats(w *bufio.Writer) {
	s.mu.Lock()
	n := len(s.items)
	s.mu.Unlock()
	io.WriteString(w, "STAT pid 4242\r\n")
	io.WriteString(w, "STAT version 1.6.0-fake\r\n")
	fmt.Fprintf(w, "STAT curr_items %d\r\n", n)
	io.WriteString(w, "END\r\n")
}
