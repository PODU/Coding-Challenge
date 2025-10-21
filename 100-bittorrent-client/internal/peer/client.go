package peer

import (
	"fmt"
	"net"
	"time"
)

// Client is a connection to one peer after the handshake.
type Client struct {
	Conn     net.Conn
	Choked   bool
	Bitfield Bitfield
	peerID   [20]byte
	infoHash [20]byte
	Addr     string
}

// NewClient dials addr, does the handshake and reads the peer's bitfield. New
// connections start out choked.
func NewClient(addr string, infoHash, peerID [20]byte, timeout time.Duration) (*Client, error) {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, fmt.Errorf("peer: dial %s: %w", addr, err)
	}
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		conn.Close()
		return nil, err
	}
	c, err := NewClientConn(conn, infoHash, peerID)
	if err != nil {
		conn.Close()
		return nil, err
	}
	// From here on callers set their own deadlines.
	_ = conn.SetDeadline(time.Time{})
	c.Addr = addr
	return c, nil
}

// NewClientConn does the handshake and bitfield exchange over conn.
func NewClientConn(conn net.Conn, infoHash, peerID [20]byte) (*Client, error) {
	hs, err := DoHandshake(conn, infoHash, peerID)
	if err != nil {
		return nil, err
	}
	c := &Client{
		Conn:     conn,
		Choked:   true,
		peerID:   hs.PeerID,
		infoHash: infoHash,
		Addr:     conn.RemoteAddr().String(),
	}
	if err := c.recvBitfield(); err != nil {
		return nil, err
	}
	return c, nil
}

// recvBitfield waits for the bitfield message that follows the handshake,
// skipping keep-alives.
func (c *Client) recvBitfield() error {
	if err := c.Conn.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		return err
	}
	defer c.Conn.SetDeadline(time.Time{})

	for {
		msg, err := ReadMessage(c.Conn)
		if err != nil {
			return fmt.Errorf("peer: read bitfield: %w", err)
		}
		if msg == nil {
			continue
		}
		if msg.ID != MsgBitfield {
			return fmt.Errorf("peer: expected Bitfield, got %s", msg)
		}
		c.Bitfield = Bitfield(msg.Payload)
		return nil
	}
}

// HasPiece reports whether the peer advertises the given piece.
func (c *Client) HasPiece(index int) bool {
	return c.Bitfield.HasPiece(index)
}

// PeerID returns the peer's 20-byte id from the handshake.
func (c *Client) PeerID() [20]byte { return c.peerID }

// Read reads one message and applies choke, unchoke, have and bitfield
// messages to c. The message is returned so the caller can handle pieces.
func (c *Client) Read() (*Message, error) {
	msg, err := ReadMessage(c.Conn)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, nil // keep-alive
	}
	switch msg.ID {
	case MsgChoke:
		c.Choked = true
	case MsgUnchoke:
		c.Choked = false
	case MsgHave:
		index, err := ParseHave(msg)
		if err != nil {
			return nil, err
		}
		c.Bitfield.SetPiece(index)
	case MsgBitfield:
		c.Bitfield = Bitfield(msg.Payload)
	}
	return msg, nil
}

func (c *Client) send(m *Message) error {
	_, err := c.Conn.Write(m.Serialize())
	return err
}

// SendInterested sends an interested message.
func (c *Client) SendInterested() error { return c.send(&Message{ID: MsgInterested}) }

// SendNotInterested sends a not-interested message.
func (c *Client) SendNotInterested() error { return c.send(&Message{ID: MsgNotInterested}) }

// SendUnchoke sends an unchoke message.
func (c *Client) SendUnchoke() error { return c.send(&Message{ID: MsgUnchoke}) }

// SendChoke sends a choke message.
func (c *Client) SendChoke() error { return c.send(&Message{ID: MsgChoke}) }

// SendRequest requests a block of a piece.
func (c *Client) SendRequest(index, begin, length int) error {
	return c.send(FormatRequest(index, begin, length))
}

// SendHave announces a completed piece.
func (c *Client) SendHave(index int) error { return c.send(FormatHave(index)) }

// Close closes the underlying connection.
func (c *Client) Close() error { return c.Conn.Close() }
