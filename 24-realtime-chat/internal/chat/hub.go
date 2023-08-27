package chat

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// DefaultRoom is where clients go when they don't ask for a room.
const DefaultRoom = "general"

type command struct {
	client *Client
	kind   string
	text   string
	room   string
}

// Hub owns all shared chat state. Only the Run goroutine touches the rooms map
// and client metadata, so there are no locks; everything else talks to it
// over channels.
type Hub struct {
	register   chan *Client
	unregister chan *Client
	inbound    chan command
	shutdown   chan struct{}
	done       chan struct{}

	shutOnce sync.Once

	rooms map[string]map[*Client]bool
}

// NewHub allocates a hub. Call Run in its own goroutine before use.
func NewHub() *Hub {
	return &Hub{
		register:   make(chan *Client),
		unregister: make(chan *Client),
		inbound:    make(chan command, 256),
		shutdown:   make(chan struct{}),
		done:       make(chan struct{}),
		rooms:      make(map[string]map[*Client]bool),
	}
}

// Run is the hub's event loop. It returns once Shutdown has closed every
// client.
func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.handleRegister(c)
		case c := <-h.unregister:
			h.handleUnregister(c, true)
		case cmd := <-h.inbound:
			h.handleCommand(cmd)
		case <-h.shutdown:
			h.handleShutdown()
			return
		}
	}
}

// Register adds a client to the hub. It returns false if the hub is shutting
// down, in which case the caller should close the connection.
func (h *Hub) Register(c *Client) bool {
	select {
	case h.register <- c:
		return true
	case <-h.shutdown:
		return false
	}
}

// Unregister removes a client. It is safe to call more than once, and during
// or after shutdown.
func (h *Hub) Unregister(c *Client) {
	select {
	case h.unregister <- c:
	case <-h.done:
	}
}

// Message submits a chat message from c to its current room.
func (h *Hub) Message(c *Client, text string) {
	h.submit(command{client: c, kind: TypeMessage, text: text})
}

// JoinRoom moves c to a different room.
func (h *Hub) JoinRoom(c *Client, room string) {
	h.submit(command{client: c, kind: TypeJoin, room: room})
}

// Quit removes c in response to an explicit quit frame.
func (h *Hub) Quit(c *Client) { h.submit(command{client: c, kind: TypeQuit}) }

func (h *Hub) submit(cmd command) {
	select {
	case h.inbound <- cmd:
	case <-h.shutdown:
	}
}

// Shutdown tells every client the server is going away, disconnects them, and
// waits for Run to return. Calling it again is a no-op.
func (h *Hub) Shutdown() {
	h.shutOnce.Do(func() { close(h.shutdown) })
	<-h.done
}

func (h *Hub) handleRegister(c *Client) {
	if c.room == "" {
		c.room = DefaultRoom
	}
	if c.baseName == "" {
		c.baseName = c.name
	}
	h.addToRoom(c)
}

// addToRoom puts c in c.room under a name that's unique there, tells c which
// name it got, and announces it to the room.
func (h *Hub) addToRoom(c *Client) {
	members := h.rooms[c.room]
	if members == nil {
		members = make(map[*Client]bool)
		h.rooms[c.room] = members
	}
	c.name = uniqueName(members, c.baseName)
	members[c] = true

	// If even the join ack can't be queued the client is hopeless; drop it
	// without announcing anything.
	if !h.sendToClient(c, Outgoing{Type: TypeJoin, Room: c.room, From: c.name, TS: now()}) {
		h.handleUnregister(c, false)
		return
	}
	h.sendToRoom(c.room, Outgoing{Type: TypeSystem, Room: c.room, Text: c.name + " joined", TS: now()}, c)
	h.sendPresence(c.room)
}

func (h *Hub) handleUnregister(c *Client, notify bool) {
	members, ok := h.rooms[c.room]
	if !ok || !members[c] {
		// Already removed, but make sure send is closed.
		c.close()
		return
	}
	delete(members, c)
	c.close()

	roomEmpty := len(members) == 0
	if roomEmpty {
		delete(h.rooms, c.room)
	}
	if notify && !roomEmpty {
		h.sendToRoom(c.room, Outgoing{Type: TypeSystem, Room: c.room, Text: c.name + " left", TS: now()}, nil)
		h.sendPresence(c.room)
	}
}

func (h *Hub) handleCommand(cmd command) {
	switch cmd.kind {
	case TypeMessage:
		c := cmd.client
		text := strings.TrimSpace(cmd.text)
		if text == "" {
			return
		}
		if !h.rooms[c.room][c] { // sender already left
			return
		}
		// The sender already shows its own message, so skip it.
		h.sendToRoom(c.room, Outgoing{Type: TypeMessage, Room: c.room, From: c.name, Text: text, TS: now()}, c)
	case TypeJoin:
		h.moveRoom(cmd.client, cmd.room)
	case TypeQuit:
		h.handleUnregister(cmd.client, true)
	}
}

func (h *Hub) moveRoom(c *Client, room string) {
	room = strings.TrimSpace(room)
	if room == "" || room == c.room {
		return
	}
	if members, ok := h.rooms[c.room]; ok && members[c] {
		delete(members, c)
		if len(members) == 0 {
			delete(h.rooms, c.room)
		} else {
			h.sendToRoom(c.room, Outgoing{Type: TypeSystem, Room: c.room, Text: c.name + " left", TS: now()}, nil)
			h.sendPresence(c.room)
		}
	}
	c.room = room
	h.addToRoom(c)
}

func (h *Hub) handleShutdown() {
	for room, members := range h.rooms {
		data, _ := json.Marshal(Outgoing{Type: TypeSystem, Room: room, Text: "server shutting down", TS: now()})
		for c := range members {
			select { // don't block on a full buffer
			case c.send <- data:
			default:
			}
			c.close()
		}
		delete(h.rooms, room)
	}
	close(h.done)
}

// sendToRoom delivers msg to every member of room except skip. Anyone whose
// buffer is full is too slow to keep up and gets disconnected, after the loop
// so the map isn't changed while we're ranging over it.
func (h *Hub) sendToRoom(room string, msg Outgoing, skip *Client) {
	members := h.rooms[room]
	if len(members) == 0 {
		return
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	var dead []*Client
	for c := range members {
		if c == skip {
			continue
		}
		select {
		case c.send <- data:
		default:
			dead = append(dead, c)
		}
	}
	for _, c := range dead {
		h.handleUnregister(c, true)
	}
}

// sendToClient returns false if c's buffer is full.
func (h *Hub) sendToClient(c *Client, msg Outgoing) bool {
	data, err := json.Marshal(msg)
	if err != nil {
		return false
	}
	select {
	case c.send <- data:
		return true
	default:
		return false
	}
}

func (h *Hub) sendPresence(room string) {
	members := h.rooms[room]
	if len(members) == 0 {
		return
	}
	names := make([]string, 0, len(members))
	for c := range members {
		names = append(names, c.name)
	}
	sort.Strings(names)
	h.sendToRoom(room, Outgoing{Type: TypePresence, Room: room, Members: names}, nil)
}

// uniqueName returns want if free in members, otherwise want-2, want-3, ...
func uniqueName(members map[*Client]bool, want string) string {
	want = strings.TrimSpace(want)
	if want == "" {
		want = "anon"
	}
	taken := make(map[string]bool, len(members))
	for c := range members {
		taken[c.name] = true
	}
	if !taken[want] {
		return want
	}
	for i := 2; ; i++ {
		cand := fmt.Sprintf("%s-%d", want, i)
		if !taken[cand] {
			return cand
		}
	}
}
