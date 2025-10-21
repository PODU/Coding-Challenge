package peer

import (
	"encoding/binary"
	"fmt"
	"io"
)

// MessageID is the type byte of a peer message.
type MessageID uint8

const (
	MsgChoke         MessageID = 0
	MsgUnchoke       MessageID = 1
	MsgInterested    MessageID = 2
	MsgNotInterested MessageID = 3
	MsgHave          MessageID = 4
	MsgBitfield      MessageID = 5
	MsgRequest       MessageID = 6
	MsgPiece         MessageID = 7
	MsgCancel        MessageID = 8
)

// MaxMessageSize caps the length prefix we accept. A 16 KiB block fits easily;
// this just stops a bad peer from making us allocate gigabytes.
const MaxMessageSize = 1 << 20

// Message is one peer message. A nil *Message is a keep-alive.
type Message struct {
	ID      MessageID
	Payload []byte
}

func (m *Message) String() string {
	if m == nil {
		return "KeepAlive"
	}
	names := map[MessageID]string{
		MsgChoke: "Choke", MsgUnchoke: "Unchoke", MsgInterested: "Interested",
		MsgNotInterested: "NotInterested", MsgHave: "Have", MsgBitfield: "Bitfield",
		MsgRequest: "Request", MsgPiece: "Piece", MsgCancel: "Cancel",
	}
	name, ok := names[m.ID]
	if !ok {
		name = fmt.Sprintf("Unknown(%d)", m.ID)
	}
	return fmt.Sprintf("%s[%d bytes]", name, len(m.Payload))
}

// Serialize returns the wire form of m: a 4-byte length, the id, then the
// payload. A nil message gives the keep-alive, four zero bytes.
func (m *Message) Serialize() []byte {
	if m == nil {
		return make([]byte, 4)
	}
	length := uint32(len(m.Payload) + 1) // +1 for the id byte
	buf := make([]byte, 4+length)
	binary.BigEndian.PutUint32(buf[:4], length)
	buf[4] = byte(m.ID)
	copy(buf[5:], m.Payload)
	return buf
}

// ReadMessage reads one message from r. A keep-alive comes back as (nil, nil).
func ReadMessage(r io.Reader) (*Message, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lenBuf)
	if length == 0 {
		return nil, nil
	}
	if length > MaxMessageSize {
		return nil, fmt.Errorf("peer: message length %d exceeds max %d", length, MaxMessageSize)
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return &Message{ID: MessageID(buf[0]), Payload: buf[1:]}, nil
}

// FormatRequest builds a request for length bytes of piece index at begin.
func FormatRequest(index, begin, length int) *Message {
	payload := make([]byte, 12)
	binary.BigEndian.PutUint32(payload[0:4], uint32(index))
	binary.BigEndian.PutUint32(payload[4:8], uint32(begin))
	binary.BigEndian.PutUint32(payload[8:12], uint32(length))
	return &Message{ID: MsgRequest, Payload: payload}
}

// FormatHave builds a have message for piece index.
func FormatHave(index int) *Message {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload, uint32(index))
	return &Message{ID: MsgHave, Payload: payload}
}

// ParseHave returns the piece index carried by a have message.
func ParseHave(m *Message) (int, error) {
	if m.ID != MsgHave {
		return 0, fmt.Errorf("peer: expected Have (id 4), got id %d", m.ID)
	}
	if len(m.Payload) != 4 {
		return 0, fmt.Errorf("peer: Have payload is %d bytes, want 4", len(m.Payload))
	}
	return int(binary.BigEndian.Uint32(m.Payload)), nil
}

// ParsePiece copies the block in a piece message into buf at its begin offset
// and returns the block size. The message must be for piece index.
func ParsePiece(index int, buf []byte, m *Message) (int, error) {
	if m.ID != MsgPiece {
		return 0, fmt.Errorf("peer: expected Piece (id 7), got id %d", m.ID)
	}
	if len(m.Payload) < 8 {
		return 0, fmt.Errorf("peer: Piece payload too short (%d bytes)", len(m.Payload))
	}
	parsedIndex := int(binary.BigEndian.Uint32(m.Payload[0:4]))
	if parsedIndex != index {
		return 0, fmt.Errorf("peer: Piece index mismatch: got %d want %d", parsedIndex, index)
	}
	begin := int(binary.BigEndian.Uint32(m.Payload[4:8]))
	if begin < 0 || begin > len(buf) {
		return 0, fmt.Errorf("peer: Piece begin offset %d out of bounds (buf %d)", begin, len(buf))
	}
	data := m.Payload[8:]
	if begin+len(data) > len(buf) {
		return 0, fmt.Errorf("peer: Piece data (%d bytes at %d) exceeds buffer %d", len(data), begin, len(buf))
	}
	copy(buf[begin:], data)
	return len(data), nil
}
