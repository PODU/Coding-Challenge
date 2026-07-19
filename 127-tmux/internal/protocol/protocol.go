// Package protocol frames messages between the cctmux client and server.
//
// Each frame is a 4-byte big-endian length, then a type byte, then the
// payload. The length counts the type byte as well as the payload.
package protocol

import (
	"encoding/binary"
	"errors"
	"io"
	"strconv"
	"strings"
)

type Type uint8

const (
	// Client -> server.
	NewSession    Type = iota + 1 // payload: session name
	AttachSession                 // payload: session name
	KillSession                   // payload: session name
	ListSessions                  // no payload
	Detach                        // no payload
	Input                         // payload: raw keystroke bytes
	Resize                        // payload: "WxH"
	Command                       // payload: a command line, e.g. `split-window -h`

	// Server -> client.
	Output      // payload: bytes to write to the client's terminal
	SessionList // payload: formatted list-sessions text
	OK          // payload: optional human text
	Error       // payload: error text
)

type Message struct {
	Type    Type
	Payload []byte
}

func (m Message) Str() string { return string(m.Payload) }

// ErrClosed is what ReadMessage returns when the peer hangs up.
var ErrClosed = io.EOF

func WriteMessage(w io.Writer, t Type, payload []byte) error {
	hdr := make([]byte, 4)
	binary.BigEndian.PutUint32(hdr, uint32(len(payload)+1))
	if _, err := w.Write(hdr); err != nil {
		return err
	}
	if _, err := w.Write([]byte{byte(t)}); err != nil {
		return err
	}
	if len(payload) > 0 {
		if _, err := w.Write(payload); err != nil {
			return err
		}
	}
	return nil
}

// ReadMessage reads one frame. A connection closed between or partway
// through a header gives io.EOF.
func ReadMessage(r io.Reader) (Message, error) {
	hdr := make([]byte, 4)
	if _, err := io.ReadFull(r, hdr); err != nil {
		if err == io.ErrUnexpectedEOF {
			err = io.EOF
		}
		return Message{}, err
	}
	n := binary.BigEndian.Uint32(hdr)
	if n == 0 {
		return Message{}, errors.New("protocol: zero-length frame")
	}
	if n > 64*1024*1024 {
		return Message{}, errors.New("protocol: frame too large")
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return Message{}, err
	}
	return Message{Type: Type(buf[0]), Payload: buf[1:]}, nil
}

// EncodeResize formats a Resize payload as "WxH".
func EncodeResize(w, h int) []byte {
	return []byte(strconv.Itoa(w) + "x" + strconv.Itoa(h))
}

func DecodeResize(p []byte) (w, h int, err error) {
	parts := strings.SplitN(string(p), "x", 2)
	if len(parts) != 2 {
		return 0, 0, errors.New("protocol: bad resize payload")
	}
	if w, err = strconv.Atoi(parts[0]); err != nil {
		return 0, 0, err
	}
	if h, err = strconv.Atoi(parts[1]); err != nil {
		return 0, 0, err
	}
	return w, h, nil
}
