package transport

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"realtimechat/internal/chat"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
	sendBuffer     = 32
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Any origin is allowed on purpose. Put a reverse proxy in front if that
	// needs restricting.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Server serves the web client, the /ws endpoint and /healthz.
type Server struct {
	hub *chat.Hub
	mux *http.ServeMux
}

// NewServer serves webFS at / unless it is nil.
func NewServer(hub *chat.Hub, webFS fs.FS) *Server {
	s := &Server{hub: hub, mux: http.NewServeMux()}
	s.mux.HandleFunc("/ws", s.handleWS)
	s.mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	if webFS != nil {
		s.mux.Handle("/", http.FileServer(http.FS(webFS)))
	}
	return s
}

// Handler returns the server's HTTP handler.
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	user := strings.TrimSpace(r.URL.Query().Get("user"))
	room := strings.TrimSpace(r.URL.Query().Get("room"))
	if room == "" {
		room = chat.DefaultRoom
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	if user == "" {
		data, _ := json.Marshal(chat.Outgoing{Type: chat.TypeError, Text: "username required"})
		_ = conn.WriteMessage(websocket.TextMessage, data)
		_ = conn.Close()
		return
	}

	c := chat.NewClient(s.hub, user, room, sendBuffer)
	if !s.hub.Register(c) { // shutting down
		_ = conn.Close()
		return
	}

	go writePump(conn, c)
	readPump(conn, c) // blocks until the connection ends
}

// readPump forwards frames from the socket to the hub until the connection
// ends, then unregisters the client.
func readPump(conn *websocket.Conn, c *chat.Client) {
	defer func() {
		c.Hub().Unregister(c)
		_ = conn.Close()
	}()

	conn.SetReadLimit(maxMessageSize)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var in chat.Incoming
		if json.Unmarshal(data, &in) != nil {
			continue
		}
		switch in.Type {
		case chat.TypeMessage:
			c.Hub().Message(c, in.Text)
		case chat.TypeJoin:
			c.Hub().JoinRoom(c, in.Room)
		case chat.TypeQuit:
			return
		}
	}
}

// writePump writes queued frames and pings the peer, until the hub closes the
// client's send channel.
func writePump(conn *websocket.Conn, c *chat.Client) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.Send():
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
