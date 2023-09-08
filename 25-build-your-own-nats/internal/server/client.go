package server

import (
	"bufio"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/codingchallenges/nats/internal/protocol"
	"github.com/codingchallenges/nats/internal/router"
)

const (
	maxPayload = 1024 * 1024 // advertised in INFO
	// A consumer that stops reading gets disconnected after this long instead
	// of blocking the publishers writing to it.
	writeTimeout   = 10 * time.Second
	readBufferSize = 32 * 1024
)

// Client is one connection. Writes are serialized by writeMu because many
// publishers may deliver to the same client at once.
type Client struct {
	srv  *Server
	conn net.Conn
	id   uint64

	reader *bufio.Reader

	writeMu sync.Mutex

	verbose bool

	subMu sync.Mutex
	subs  map[string]*router.Subscription // keyed by sid

	closeOnce sync.Once
}

func newClient(srv *Server, conn net.Conn, id uint64) *Client {
	return &Client{
		srv:     srv,
		conn:    conn,
		id:      id,
		reader:  bufio.NewReaderSize(conn, readBufferSize),
		verbose: true, // until CONNECT says otherwise
		subs:    make(map[string]*router.Subscription),
	}
}

// readLoop sends INFO and then handles commands until the connection ends.
func (c *Client) readLoop() {
	c.write(protocol.EncodeInfo(c.srv.info(c.id)))

	for {
		line, err := c.reader.ReadBytes('\n')
		if err != nil || !c.dispatch(line) {
			c.close()
			return
		}
	}
}

// dispatch handles one control line and returns false if the connection
// should be closed.
func (c *Client) dispatch(line []byte) bool {
	cmd, err := protocol.Parse(line)
	if errors.Is(err, protocol.ErrEmptyLine) {
		return true
	}
	if err != nil {
		// Bad arguments get the same reply as an unknown verb.
		c.write(protocol.EncodeErr("Unknown Protocol Operation"))
		return true
	}

	switch cmd := cmd.(type) {
	case protocol.ConnectCmd:
		c.verbose = cmd.Options.Verbose
		c.ok()
	case protocol.PingCmd:
		c.write(protocol.Pong())
	case protocol.PongCmd:
	case protocol.SubCmd:
		c.handleSub(cmd)
		c.ok()
	case protocol.UnsubCmd:
		c.handleUnsub(cmd)
		c.ok()
	case protocol.PubCmd:
		return c.handlePub(cmd)
	}
	return true
}

// handleSub adds a subscription, replacing any existing one with the same sid.
func (c *Client) handleSub(cmd protocol.SubCmd) {
	sub := &router.Subscription{
		Subject: cmd.Subject,
		Queue:   cmd.Queue,
		Sid:     cmd.Sid,
		Sub:     c,
	}
	c.subMu.Lock()
	if old, ok := c.subs[cmd.Sid]; ok {
		c.srv.registry.Unsubscribe(old)
	}
	c.subs[cmd.Sid] = sub
	c.subMu.Unlock()

	c.srv.registry.Subscribe(sub)
}

// handleUnsub removes a subscription now, or after max_msgs more messages.
func (c *Client) handleUnsub(cmd protocol.UnsubCmd) {
	c.subMu.Lock()
	sub, ok := c.subs[cmd.Sid]
	if !ok {
		c.subMu.Unlock()
		return
	}
	if cmd.HasMax && cmd.MaxMsgs > 0 {
		c.subMu.Unlock()
		c.srv.registry.SetAutoUnsub(sub, cmd.MaxMsgs)
		return
	}
	delete(c.subs, cmd.Sid)
	c.subMu.Unlock()
	c.srv.registry.Unsubscribe(sub)
}

// handlePub reads the payload and publishes it. It returns false if the
// connection should be closed.
func (c *Client) handlePub(cmd protocol.PubCmd) bool {
	if cmd.Size > maxPayload {
		c.write(protocol.EncodeErr("Maximum Payload Violation"))
		return false
	}
	payload := make([]byte, cmd.Size)
	if _, err := io.ReadFull(c.reader, payload); err != nil {
		return false
	}
	if !c.consumeCRLF() {
		c.write(protocol.EncodeErr("Invalid Publish Size"))
		return false
	}

	c.srv.registry.Publish(cmd.Subject, cmd.Reply, payload)
	c.ok()
	return true
}

// consumeCRLF reads the CRLF that must follow a payload.
func (c *Client) consumeCRLF() bool {
	var tail [2]byte
	if _, err := io.ReadFull(c.reader, tail[:]); err != nil {
		return false
	}
	return tail[0] == '\r' && tail[1] == '\n'
}

// Deliver sends a MSG to this client.
func (c *Client) Deliver(subject, sid, reply string, payload []byte) {
	c.write(protocol.EncodeMsg(subject, sid, reply, payload))
}

// ok sends +OK if the client asked for verbose mode.
func (c *Client) ok() {
	if c.verbose {
		c.write(protocol.OK())
	}
}

// write sends b. Any write error closes the connection.
func (c *Client) write(b []byte) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	if _, err := c.conn.Write(b); err != nil {
		// Close from another goroutine so we don't do it while holding writeMu.
		go c.close()
	}
}

// takeAllSubs removes and returns all of this client's subscriptions.
func (c *Client) takeAllSubs() []*router.Subscription {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	out := make([]*router.Subscription, 0, len(c.subs))
	for _, s := range c.subs {
		out = append(out, s)
	}
	c.subs = make(map[string]*router.Subscription)
	return out
}

// close closes the connection. It is safe to call more than once.
func (c *Client) close() {
	c.closeOnce.Do(func() {
		_ = c.conn.Close()
	})
}
