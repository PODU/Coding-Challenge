package client

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"time"

	"dhcpclient/internal/dhcp"
)

// State is a DORA state-machine state.
type State int

const (
	StateInit State = iota
	StateSelecting
	StateRequesting
	StateBound
	StateReleased
	StateFailed
)

func (s State) String() string {
	switch s {
	case StateInit:
		return "INIT"
	case StateSelecting:
		return "SELECTING"
	case StateRequesting:
		return "REQUESTING"
	case StateBound:
		return "BOUND"
	case StateReleased:
		return "RELEASED"
	case StateFailed:
		return "FAILED"
	default:
		return fmt.Sprintf("State(%d)", int(s))
	}
}

// Event is an input to the DORA state machine.
type Event int

const (
	EventStart Event = iota
	EventOffer
	EventAck
	EventNak
	EventTimeout
	EventRelease
)

func (e Event) String() string {
	switch e {
	case EventStart:
		return "START"
	case EventOffer:
		return "OFFER"
	case EventAck:
		return "ACK"
	case EventNak:
		return "NAK"
	case EventTimeout:
		return "TIMEOUT"
	case EventRelease:
		return "RELEASE"
	default:
		return fmt.Sprintf("Event(%d)", int(e))
	}
}

// Action is the side effect a transition asks the runner to perform.
type Action int

const (
	ActionNone Action = iota
	ActionSendDiscover
	ActionSendRequest
	ActionBound
	ActionRestart
	ActionSendRelease
	ActionFail
)

func (a Action) String() string {
	switch a {
	case ActionNone:
		return "NONE"
	case ActionSendDiscover:
		return "SEND_DISCOVER"
	case ActionSendRequest:
		return "SEND_REQUEST"
	case ActionBound:
		return "BOUND"
	case ActionRestart:
		return "RESTART"
	case ActionSendRelease:
		return "SEND_RELEASE"
	case ActionFail:
		return "FAIL"
	default:
		return fmt.Sprintf("Action(%d)", int(a))
	}
}

// Transition returns the next state and the action for ev. Pairs it doesn't
// handle keep the state and return ActionNone.
func Transition(state State, ev Event) (State, Action) {
	switch state {
	case StateInit:
		if ev == EventStart {
			return StateSelecting, ActionSendDiscover
		}
	case StateSelecting:
		switch ev {
		case EventOffer:
			return StateRequesting, ActionSendRequest
		case EventTimeout:
			return StateSelecting, ActionSendDiscover
		}
	case StateRequesting:
		switch ev {
		case EventAck:
			return StateBound, ActionBound
		case EventNak:
			return StateInit, ActionRestart
		case EventTimeout:
			return StateRequesting, ActionSendRequest
		}
	case StateBound:
		if ev == EventRelease {
			return StateReleased, ActionSendRelease
		}
	}
	return state, ActionNone
}

var (
	// ErrNoResponse means retransmissions ran out without an OFFER or ACK.
	ErrNoResponse = errors.New("dhcp: no response from server (timed out)")
	// ErrRejected means the server kept NAKing until MaxRestarts ran out.
	ErrRejected = errors.New("dhcp: request rejected by server (NAK)")
)

// errNak tells Run to start over from INIT.
var errNak = errors.New("dhcp: nak")

type Lease struct {
	IP           net.IP
	ServerID     net.IP
	SubnetMask   net.IP
	Routers      []net.IP
	DNS          []net.IP
	LeaseSeconds uint32
	XID          uint32
	MAC          net.HardwareAddr
	Acquired     time.Time
}

// Client runs the DORA handshake. Transport and MAC are required; zero values
// elsewhere get defaults.
type Client struct {
	Transport Transport
	MAC       net.HardwareAddr
	XID       uint32
	Hostname  string
	RequestIP net.IP

	Timeout     time.Duration // first retransmit wait; doubles each attempt
	Retries     int           // retransmissions per phase
	MaxBackoff  time.Duration
	MaxRestarts int // restarts after a NAK

	Now  func() time.Time
	Rand *rand.Rand // used for xid and jitter; nil means no jitter
	Logf func(format string, args ...any)
}

func (c *Client) applyDefaults() {
	if c.Timeout <= 0 {
		c.Timeout = 4 * time.Second
	}
	if c.Retries <= 0 {
		c.Retries = 4
	}
	if c.MaxBackoff <= 0 {
		c.MaxBackoff = 64 * time.Second
	}
	if c.MaxRestarts <= 0 {
		c.MaxRestarts = 2
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	if c.Logf == nil {
		c.Logf = func(string, ...any) {}
	}
	if c.XID == 0 && c.Rand != nil {
		c.XID = c.Rand.Uint32()
	}
}

// Run performs DORA and returns the lease. A NAK restarts from INIT, up to
// MaxRestarts times.
func (c *Client) Run() (*Lease, error) {
	c.applyDefaults()
	restarts := 0
	for {
		lease, err := c.runOnce()
		if errors.Is(err, errNak) {
			restarts++
			if restarts > c.MaxRestarts {
				c.Logf("[INIT] NAK restart budget (%d) exhausted", c.MaxRestarts)
				return nil, ErrRejected
			}
			c.Logf("[INIT] received NAK, restarting DORA (restart %d/%d)", restarts, c.MaxRestarts)
			if c.Rand != nil {
				c.XID = c.Rand.Uint32()
			}
			continue
		}
		return lease, err
	}
}

func (c *Client) runOnce() (*Lease, error) {
	offer, err := c.selecting()
	if err != nil {
		return nil, err
	}
	return c.requesting(offer)
}

func (c *Client) selecting() (*dhcp.Message, error) {
	disc := dhcp.BuildDiscover(c.MAC, c.XID, c.RequestIP, c.Hostname)
	raw, err := disc.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshal DISCOVER: %w", err)
	}
	accept := map[uint8]bool{dhcp.OFFER: true}
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if err := c.Transport.Send(raw); err != nil {
			return nil, fmt.Errorf("send DISCOVER: %w", err)
		}
		c.Logf("[SELECTING] sent DISCOVER xid=0x%08x attempt=%d", c.XID, attempt)
		deadline := c.Now().Add(c.backoff(attempt))
		msg, err := c.waitFor(deadline, accept)
		if errors.Is(err, ErrTimeout) {
			continue
		}
		if err != nil {
			return nil, err
		}
		c.Logf("[SELECTING] received OFFER yiaddr=%s server=%s", msg.YIAddr, msg.ServerID())
		return msg, nil
	}
	return nil, ErrNoResponse
}

func (c *Client) requesting(offer *dhcp.Message) (*Lease, error) {
	req := dhcp.BuildRequest(c.MAC, c.XID, offer.YIAddr, offer.ServerID(), c.Hostname)
	raw, err := req.Marshal()
	if err != nil {
		return nil, fmt.Errorf("marshal REQUEST: %w", err)
	}
	accept := map[uint8]bool{dhcp.ACK: true, dhcp.NAK: true}
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if err := c.Transport.Send(raw); err != nil {
			return nil, fmt.Errorf("send REQUEST: %w", err)
		}
		c.Logf("[REQUESTING] sent REQUEST xid=0x%08x requested=%s attempt=%d", c.XID, offer.YIAddr, attempt)
		deadline := c.Now().Add(c.backoff(attempt))
		msg, err := c.waitFor(deadline, accept)
		if errors.Is(err, ErrTimeout) {
			continue
		}
		if err != nil {
			return nil, err
		}
		mt, _ := msg.MessageType()
		if mt == dhcp.NAK {
			c.Logf("[REQUESTING] received NAK")
			return nil, errNak
		}
		c.Logf("[REQUESTING] received ACK yiaddr=%s", msg.YIAddr)
		return c.leaseFromAck(msg), nil
	}
	return nil, ErrNoResponse
}

// waitFor returns the first reply with our xid and a message type in accept,
// dropping anything else, or ErrTimeout at the deadline.
func (c *Client) waitFor(deadline time.Time, accept map[uint8]bool) (*dhcp.Message, error) {
	for {
		if !c.Now().Before(deadline) {
			return nil, ErrTimeout
		}
		raw, err := c.Transport.Receive(deadline)
		if errors.Is(err, ErrTimeout) {
			return nil, ErrTimeout
		}
		if err != nil {
			return nil, fmt.Errorf("receive: %w", err)
		}
		msg, err := dhcp.Unmarshal(raw)
		if err != nil {
			c.Logf("[recv] discarding undecodable packet: %v", err)
			continue
		}
		if msg.Op != dhcp.BootReply {
			continue
		}
		if msg.XID != c.XID {
			c.Logf("[recv] discarding packet with xid=0x%08x (want 0x%08x)", msg.XID, c.XID)
			continue
		}
		mt, ok := msg.MessageType()
		if !ok {
			c.Logf("[recv] discarding packet with no message type (Option 53)")
			continue
		}
		if !accept[mt] {
			c.Logf("[recv] discarding unexpected message type %d", mt)
			continue
		}
		return msg, nil
	}
}

func (c *Client) leaseFromAck(ack *dhcp.Message) *Lease {
	leaseSecs, _ := ack.LeaseTime()
	return &Lease{
		IP:           ack.YIAddr,
		ServerID:     ack.ServerID(),
		SubnetMask:   ack.SubnetMask(),
		Routers:      ack.Routers(),
		DNS:          ack.DNS(),
		LeaseSeconds: leaseSecs,
		XID:          ack.XID,
		MAC:          c.MAC,
		Acquired:     c.Now(),
	}
}

// Release sends a DHCPRELEASE for lease.
func (c *Client) Release(lease *Lease) error {
	c.applyDefaults()
	rel := dhcp.BuildRelease(c.MAC, lease.XID, lease.IP, lease.ServerID)
	raw, err := rel.Marshal()
	if err != nil {
		return fmt.Errorf("marshal RELEASE: %w", err)
	}
	c.Logf("[BOUND] sending RELEASE for %s", lease.IP)
	if err := c.Transport.Send(raw); err != nil {
		return fmt.Errorf("send RELEASE: %w", err)
	}
	return nil
}

// backoff returns Timeout * 2^attempt, capped at MaxBackoff, with up to one
// second of jitter either way when Rand is set (as RFC 2131 suggests).
func (c *Client) backoff(attempt int) time.Duration {
	d := c.Timeout
	for i := 0; i < attempt; i++ {
		d *= 2
		if d >= c.MaxBackoff {
			d = c.MaxBackoff
			break
		}
	}
	if c.Rand != nil {
		jitter := time.Duration(c.Rand.Int63n(int64(2*time.Second))) - time.Second
		d += jitter
		if d < c.Timeout {
			d = c.Timeout
		}
	}
	return d
}
