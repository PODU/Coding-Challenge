package client

import (
	"bufio"
	"net"
	"reflect"
	"testing"

	"rediscli/resp"
)

// fakeServer spins a real net.Listener that speaks RESP: it decodes each
// request and replies using a handler. It is fully in-process and deterministic
// (no external redis-server needed).
type fakeServer struct {
	ln      net.Listener
	handler func(args []string) resp.Value
}

func newFakeServer(t *testing.T, handler func(args []string) resp.Value) *fakeServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	s := &fakeServer{ln: ln, handler: handler}
	go s.serve()
	t.Cleanup(func() { ln.Close() })
	return s
}

func (s *fakeServer) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go func(c net.Conn) {
			defer c.Close()
			r := bufio.NewReader(c)
			for {
				req, err := resp.Decode(r)
				if err != nil {
					return
				}
				args := make([]string, len(req.Array))
				for i, a := range req.Array {
					args[i] = a.Str
				}
				if _, err := c.Write(resp.Marshal(s.handler(args))); err != nil {
					return
				}
			}
		}(conn)
	}
}

func (s *fakeServer) addr() (string, string) {
	host, port, _ := net.SplitHostPort(s.ln.Addr().String())
	return host, port
}

func dialFake(t *testing.T, s *fakeServer) *Client {
	t.Helper()
	host, port := s.addr()
	c, err := Dial(host, port)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestSendRoundTrip(t *testing.T) {
	handler := func(args []string) resp.Value {
		switch args[0] {
		case "PING":
			return resp.Value{Type: resp.SimpleString, Str: "PONG"}
		case "SET":
			return resp.Value{Type: resp.SimpleString, Str: "OK"}
		case "GET":
			return resp.Value{Type: resp.BulkString, IsNull: true} // miss
		case "LRANGE":
			return resp.Value{Type: resp.Array, Array: []resp.Value{
				{Type: resp.BulkString, Str: "a"},
				{Type: resp.BulkString, Str: "b"},
			}}
		default:
			return resp.Value{Type: resp.Error, Str: "ERR unknown command '" + args[0] + "'"}
		}
	}
	s := newFakeServer(t, handler)
	c := dialFake(t, s)

	tests := []struct {
		args []string
		want resp.Value
	}{
		{[]string{"PING"}, resp.Value{Type: resp.SimpleString, Str: "PONG"}},
		{[]string{"SET", "name", "John"}, resp.Value{Type: resp.SimpleString, Str: "OK"}},
		{[]string{"GET", "missing"}, resp.Value{Type: resp.BulkString, IsNull: true}},
		{[]string{"LRANGE", "k", "0", "-1"}, resp.Value{Type: resp.Array, Array: []resp.Value{
			{Type: resp.BulkString, Str: "a"},
			{Type: resp.BulkString, Str: "b"},
		}}},
		{[]string{"BOGUS"}, resp.Value{Type: resp.Error, Str: "ERR unknown command 'BOGUS'"}},
	}
	for _, tt := range tests {
		got, err := c.Send(tt.args)
		if err != nil {
			t.Fatalf("Send(%v) error: %v", tt.args, err)
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Send(%v) = %+v, want %+v", tt.args, got, tt.want)
		}
	}
}

func TestSendPreservesArgsAcrossSocket(t *testing.T) {
	var captured []string
	s := newFakeServer(t, func(args []string) resp.Value {
		captured = args
		return resp.Value{Type: resp.SimpleString, Str: "OK"}
	})
	c := dialFake(t, s)

	if _, err := c.Send([]string{"SET", "k", "two words"}); err != nil {
		t.Fatalf("Send error: %v", err)
	}
	want := []string{"SET", "k", "two words"}
	if !reflect.DeepEqual(captured, want) {
		t.Errorf("server received %v, want %v", captured, want)
	}
}

func TestDialFailure(t *testing.T) {
	// Port 1 is reserved and should refuse/never accept.
	if _, err := Dial("127.0.0.1", "1"); err == nil {
		t.Error("Dial to closed port expected error, got nil")
	}
}
