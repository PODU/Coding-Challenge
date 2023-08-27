package chat

import "sync"

// DefaultSendBuffer is the per-client outbound queue size. A client that lets
// it fill up is disconnected rather than allowed to stall the hub.
const DefaultSendBuffer = 32

// Client is the hub's view of a connection: a send queue plus some metadata
// that only the hub goroutine writes. The socket handling lives in the
// transport package.
type Client struct {
	hub *Hub

	// name may have a "-2" style suffix to keep it unique in the room;
	// baseName is what the user asked for, reused when they change rooms.
	name     string
	baseName string
	room     string

	send      chan []byte
	closeOnce sync.Once
}

// NewClient creates a client for hub. buffer <= 0 means DefaultSendBuffer.
func NewClient(hub *Hub, name, room string, buffer int) *Client {
	if buffer <= 0 {
		buffer = DefaultSendBuffer
	}
	return &Client{
		hub:      hub,
		name:     name,
		baseName: name,
		room:     room,
		send:     make(chan []byte, buffer),
	}
}

// Send returns the outbound queue. The hub closes it when the client is
// removed.
func (c *Client) Send() <-chan []byte { return c.send }

// Hub returns the hub c belongs to.
func (c *Client) Hub() *Hub { return c.hub }

// close may be called more than once.
func (c *Client) close() {
	c.closeOnce.Do(func() { close(c.send) })
}
