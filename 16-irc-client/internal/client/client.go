// Package client keeps the IRC session state. It turns server messages into
// events for the UI and user input into protocol lines.
package client

import (
	"fmt"
	"strings"
	"sync"

	"ircclient/internal/parser"
)

// Sender is what the client needs from the connection; *conn.Conn has it.
type Sender interface {
	Send(line string) error
}

type Config struct {
	Nick    string
	User    string
	Real    string
	Channel string // joined automatically once registered, if set
	Debug   bool   // show raw protocol lines, including PING/PONG
}

// Kind decides how the UI styles an Event.
type Kind int

const (
	KindInfo     Kind = iota // server/status notices, "* ..." lines
	KindMessage              // channel PRIVMSG: "Nick: text"
	KindSelf                 // our own outgoing message echo
	KindPM                   // private message: "[PM] Nick: text"
	KindAction               // CTCP ACTION: "* Nick does X"
	KindJoin                 // "* X has joined #cc"
	KindPart                 // "* X has left #cc"
	KindQuitMsg              // "* X has left IRC (...)"
	KindNick                 // "* Old is now known as New"
	KindNotice               // NOTICE lines
	KindError                // "! ..." errors
	KindRaw                  // debug raw passthrough
	KindShutdown             // not displayed; tells main to exit
)

// Event is one line for the UI.
type Event struct {
	Kind Kind
	Text string
}

type Client struct {
	cfg  Config
	send Sender

	mu         sync.Mutex
	nick       string
	channel    string
	server     string
	registered bool
}

func New(cfg Config, s Sender) *Client {
	return &Client{cfg: cfg, send: s, nick: cfg.Nick}
}

func (c *Client) Nick() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.nick
}

// Channel returns the active channel, or "".
func (c *Client) Channel() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.channel
}

// Registered reports whether the 001 welcome has been received.
func (c *Client) Registered() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.registered
}

// Register sends NICK and USER.
func (c *Client) Register() error {
	if err := c.send.Send(parser.Encode("NICK", c.cfg.Nick)); err != nil {
		return err
	}
	return c.send.Send(parser.Encode("USER", c.cfg.User, "0", "*", c.cfg.Real))
}

// Handle updates the session for one server message, replies where the
// protocol needs it (PING), and returns what to display.
func (c *Client) Handle(m parser.Message) []Event {
	switch m.Command {
	case "PING":
		_ = c.send.Send(parser.Encode("PONG", m.Trailing()))
		if c.cfg.Debug {
			return []Event{{Kind: KindRaw, Text: "<< " + m.Raw}}
		}
		return nil

	case "PONG":
		if c.cfg.Debug {
			return []Event{{Kind: KindRaw, Text: "<< " + m.Raw}}
		}
		return nil

	case "001":
		c.mu.Lock()
		c.registered = true
		c.server = m.Prefix
		if len(m.Params) > 0 {
			c.nick = m.Params[0] // the server may have changed or truncated it
		}
		server, nick := c.server, c.nick
		c.mu.Unlock()
		if c.cfg.Channel != "" {
			_ = c.send.Send(parser.Encode("JOIN", c.cfg.Channel))
		}
		return []Event{{Kind: KindInfo, Text: fmt.Sprintf("* Connected to %s as %s", server, nick)}}

	case "JOIN":
		ch := joinTarget(m)
		if c.isSelf(m.Nick) {
			c.mu.Lock()
			c.channel = ch
			c.mu.Unlock()
		}
		return []Event{{Kind: KindJoin, Text: fmt.Sprintf("* %s has joined %s", m.Nick, ch)}}

	case "PART":
		ch := m.Target()
		reason := ""
		if len(m.Params) > 1 {
			reason = " (" + m.Trailing() + ")"
		}
		if c.isSelf(m.Nick) {
			c.mu.Lock()
			if c.channel == ch {
				c.channel = ""
			}
			c.mu.Unlock()
		}
		return []Event{{Kind: KindPart, Text: fmt.Sprintf("* %s has left %s%s", m.Nick, ch, reason)}}

	case "NICK":
		newNick := m.Trailing()
		if c.isSelf(m.Nick) {
			c.mu.Lock()
			c.nick = newNick
			c.mu.Unlock()
		}
		return []Event{{Kind: KindNick, Text: fmt.Sprintf("* %s is now known as %s", m.Nick, newNick)}}

	case "QUIT":
		return []Event{{Kind: KindQuitMsg, Text: fmt.Sprintf("* %s has left IRC (%s)", m.Nick, m.Trailing())}}

	case "PRIVMSG":
		return []Event{c.renderPrivmsg(m)}

	case "NOTICE":
		from := m.Nick
		if from == "" {
			from = m.Prefix
		}
		return []Event{{Kind: KindNotice, Text: fmt.Sprintf("-%s- %s", from, m.Trailing())}}

	case "ERROR":
		return []Event{
			{Kind: KindError, Text: "! Server closed link: " + m.Trailing()},
			{Kind: KindShutdown},
		}
	}

	if m.IsNumeric() {
		return []Event{c.renderNumeric(m)}
	}

	// Show anything unrecognised as-is rather than dropping it.
	return []Event{{Kind: KindInfo, Text: m.Raw}}
}

func (c *Client) renderPrivmsg(m parser.Message) Event {
	text := m.Trailing()
	if action, ok := ctcpAction(text); ok {
		return Event{Kind: KindAction, Text: fmt.Sprintf("* %s %s", m.Nick, action)}
	}
	// Other CTCP requests: just drop the \x01 markers.
	text = stripCTCP(text)
	if c.isSelf(m.Target()) {
		return Event{Kind: KindPM, Text: fmt.Sprintf("[PM] %s: %s", m.Nick, text)}
	}
	return Event{Kind: KindMessage, Text: fmt.Sprintf("%s: %s", m.Nick, text)}
}

func (c *Client) renderNumeric(m parser.Message) Event {
	if len(m.Command) == 3 && (m.Command[0] == '4' || m.Command[0] == '5') {
		return Event{Kind: KindError, Text: fmt.Sprintf("! %s: %s", m.Command, m.Trailing())}
	}
	return Event{Kind: KindInfo, Text: m.Trailing()}
}

// HandleInput handles one line typed by the user: a /command or a message
// for the active channel.
func (c *Client) HandleInput(line string) ([]Event, error) {
	line = strings.TrimRight(line, "\r\n")
	if strings.TrimSpace(line) == "" {
		return nil, nil
	}

	if !strings.HasPrefix(line, "/") {
		return c.sendToChannel(line)
	}

	cmd, args := splitCommand(line[1:])
	switch strings.ToLower(cmd) {
	case "join", "j":
		if args == "" {
			return errEvent("usage: /join #channel"), nil
		}
		ch := strings.Fields(args)[0]
		return nil, c.send.Send(parser.Encode("JOIN", ch))

	case "part", "leave":
		ch := strings.TrimSpace(args)
		if ch == "" {
			ch = c.Channel()
		}
		if ch == "" {
			return errEvent("no active channel to part"), nil
		}
		return nil, c.send.Send(parser.Encode("PART", ch))

	case "nick":
		if args == "" {
			return errEvent("usage: /nick NewNick"), nil
		}
		return nil, c.send.Send(parser.Encode("NICK", strings.Fields(args)[0]))

	case "msg", "privmsg":
		target, text := splitCommand(args)
		if target == "" || text == "" {
			return errEvent("usage: /msg <target> <text>"), nil
		}
		if err := c.send.Send(parser.Encode("PRIVMSG", target, text)); err != nil {
			return nil, err
		}
		return []Event{{Kind: KindSelf, Text: fmt.Sprintf("-> %s: %s", target, text)}}, nil

	case "me":
		ch := c.Channel()
		if ch == "" {
			return errEvent("no active channel"), nil
		}
		if args == "" {
			return errEvent("usage: /me <action>"), nil
		}
		payload := "\x01ACTION " + args + "\x01"
		if err := c.send.Send(parser.Encode("PRIVMSG", ch, payload)); err != nil {
			return nil, err
		}
		return []Event{{Kind: KindAction, Text: fmt.Sprintf("* %s %s", c.Nick(), args)}}, nil

	case "raw", "quote":
		if args == "" {
			return errEvent("usage: /raw <line>"), nil
		}
		return nil, c.send.Send(args)

	case "quit", "exit":
		reason := strings.TrimSpace(args)
		if reason == "" {
			reason = "Coding Challenges IRC client"
		}
		_ = c.send.Send(parser.Encode("QUIT", reason))
		return []Event{{Kind: KindShutdown}}, nil

	case "help":
		return helpEvents(), nil

	default:
		return errEvent("unknown command: /" + cmd), nil
	}
}

func (c *Client) sendToChannel(text string) ([]Event, error) {
	ch := c.Channel()
	if ch == "" {
		return errEvent("no active channel - use /join #channel first"), nil
	}
	if err := c.send.Send(parser.Encode("PRIVMSG", ch, text)); err != nil {
		return nil, err
	}
	return []Event{{Kind: KindSelf, Text: fmt.Sprintf("%s: %s", c.Nick(), text)}}, nil
}

func (c *Client) isSelf(nick string) bool {
	return strings.EqualFold(nick, c.Nick())
}

// joinTarget returns the channel of a JOIN. The parser has already folded
// "JOIN :#cc" into the same params as "JOIN #cc".
func joinTarget(m parser.Message) string {
	if len(m.Params) == 0 {
		return ""
	}
	return m.Params[0]
}

func ctcpAction(text string) (string, bool) {
	const p = "\x01ACTION "
	if !strings.HasPrefix(text, p) {
		return "", false
	}
	return strings.TrimSuffix(text[len(p):], "\x01"), true
}

func stripCTCP(text string) string {
	return strings.ReplaceAll(text, "\x01", "")
}

func splitCommand(s string) (head, rest string) {
	s = strings.TrimLeft(s, " ")
	if sp := strings.IndexByte(s, ' '); sp >= 0 {
		return s[:sp], strings.TrimLeft(s[sp+1:], " ")
	}
	return s, ""
}

func errEvent(text string) []Event {
	return []Event{{Kind: KindError, Text: "! " + text}}
}

func helpEvents() []Event {
	lines := []string{
		"Commands:",
		"  /join #channel        join a channel",
		"  /part [#channel]      leave the active (or named) channel",
		"  /nick NewNick         change nickname",
		"  /msg <target> <text>  private or channel message",
		"  /me <action>          send a CTCP action",
		"  /raw <line>           send a raw protocol line",
		"  /quit [message]       disconnect and exit",
		"  <text>                send to the active channel",
	}
	evs := make([]Event, len(lines))
	for i, l := range lines {
		evs[i] = Event{Kind: KindInfo, Text: l}
	}
	return evs
}
