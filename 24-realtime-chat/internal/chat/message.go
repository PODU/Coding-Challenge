package chat

import "time"

// Values of the "type" field on the wire.
const (
	TypeMessage  = "message"
	TypeSystem   = "system"
	TypePresence = "presence"
	TypeJoin     = "join"
	TypeQuit     = "quit"
	TypeError    = "error"
)

// Incoming is a frame from a client.
type Incoming struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	Room string `json:"room,omitempty"`
}

// Outgoing is a frame to a client.
type Outgoing struct {
	Type    string   `json:"type"`
	Room    string   `json:"room,omitempty"`
	From    string   `json:"from,omitempty"`
	Text    string   `json:"text,omitempty"`
	TS      int64    `json:"ts,omitempty"`
	Members []string `json:"members,omitempty"`
}

func now() int64 { return time.Now().Unix() }
