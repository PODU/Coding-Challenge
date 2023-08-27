package transport

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"realtimechat/internal/chat"
)

// testServer starts an in-process HTTP server backed by a running hub.
func testServer(t *testing.T) (*chat.Hub, *httptest.Server) {
	t.Helper()
	hub := chat.NewHub()
	go hub.Run()
	s := httptest.NewServer(NewServer(hub, nil).Handler())
	t.Cleanup(func() {
		s.Close()
		hub.Shutdown()
	})
	return hub, s
}

// dial opens a WebSocket to the test server for the given user/room.
func dial(t *testing.T, s *httptest.Server, user, room string) *websocket.Conn {
	t.Helper()
	u := "ws" + strings.TrimPrefix(s.URL, "http") + "/ws?user=" + user + "&room=" + room
	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func readFrame(t *testing.T, conn *websocket.Conn) chat.Outgoing {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var out chat.Outgoing
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

func readUntil(t *testing.T, conn *websocket.Conn, typ string) chat.Outgoing {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read waiting for %q: %v", typ, err)
		}
		var out chat.Outgoing
		if json.Unmarshal(data, &out) == nil && out.Type == typ {
			return out
		}
	}
}

// two dialers join general; A sends, B receives message from A; A does not.
func TestWSBroadcast(t *testing.T) {
	_, s := testServer(t)
	a := dial(t, s, "alice", "general")
	b := dial(t, s, "bob", "general")

	// Drain join/presence noise on B before A sends.
	readUntil(t, b, chat.TypePresence)

	if err := a.WriteMessage(websocket.TextMessage, mustJSON(chat.Incoming{Type: chat.TypeMessage, Text: "hi"})); err != nil {
		t.Fatal(err)
	}

	got := readUntil(t, b, chat.TypeMessage)
	if got.From != "alice" || got.Text != "hi" {
		t.Fatalf("B got %+v, want from=alice text=hi", got)
	}

	// A must not receive its own message.
	_ = a.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	for {
		_, data, err := a.ReadMessage()
		if err != nil {
			break // timeout => good, no message frame from self
		}
		var out chat.Outgoing
		if json.Unmarshal(data, &out) == nil && out.Type == chat.TypeMessage {
			t.Fatalf("sender received its own message: %+v", out)
		}
	}
}

// join/leave produces system + presence frames to the other peer.
func TestWSPresence(t *testing.T) {
	_, s := testServer(t)
	a := dial(t, s, "alice", "general")
	readUntil(t, a, chat.TypePresence) // alice alone

	_ = dial(t, s, "bob", "general")

	sys := readUntil(t, a, chat.TypeSystem)
	if sys.Text != "bob joined" {
		t.Fatalf("system = %q, want 'bob joined'", sys.Text)
	}
	pres := readUntil(t, a, chat.TypePresence)
	if len(pres.Members) != 2 {
		t.Fatalf("presence members = %v, want 2", pres.Members)
	}
}

// quit frame closes cleanly; peer sees leave notice.
func TestWSQuit(t *testing.T) {
	_, s := testServer(t)
	a := dial(t, s, "alice", "general")
	b := dial(t, s, "bob", "general")
	// Drain past bob's join so the next system frame is the leave notice.
	readUntilMembers(t, a, 2)

	if err := b.WriteMessage(websocket.TextMessage, mustJSON(chat.Incoming{Type: chat.TypeQuit})); err != nil {
		t.Fatal(err)
	}

	sys := readUntil(t, a, chat.TypeSystem)
	if sys.Text != "bob left" {
		t.Fatalf("system = %q, want 'bob left'", sys.Text)
	}
}

// An oversized frame closes the connection with a protocol error.
func TestWSOversizeClosed(t *testing.T) {
	_, s := testServer(t)
	a := dial(t, s, "alice", "general")
	readUntil(t, a, chat.TypePresence)

	big := strings.Repeat("x", maxMessageSize+1024)
	_ = a.WriteMessage(websocket.TextMessage, mustJSON(chat.Incoming{Type: chat.TypeMessage, Text: big}))

	_ = a.SetReadDeadline(time.Now().Add(2 * time.Second))
	for {
		if _, _, err := a.ReadMessage(); err != nil {
			return // connection closed as expected
		}
	}
}

// Shutdown closes every live connection.
func TestWSGracefulShutdown(t *testing.T) {
	hub, s := testServer(t)
	a := dial(t, s, "alice", "general")
	b := dial(t, s, "bob", "general")
	readUntil(t, a, chat.TypePresence)
	readUntil(t, b, chat.TypePresence)

	hub.Shutdown()

	for _, c := range []*websocket.Conn{a, b} {
		_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
		closed := false
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				closed = true
				break
			}
		}
		if !closed {
			t.Fatal("dialer was not closed on shutdown")
		}
	}
}

// readUntilMembers reads frames until a presence frame with n members arrives.
func readUntilMembers(t *testing.T, conn *websocket.Conn, n int) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read waiting for %d members: %v", n, err)
		}
		var out chat.Outgoing
		if json.Unmarshal(data, &out) == nil && out.Type == chat.TypePresence && len(out.Members) == n {
			return
		}
	}
}

func mustJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
