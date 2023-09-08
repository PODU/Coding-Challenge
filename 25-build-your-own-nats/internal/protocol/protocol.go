// Package protocol parses and encodes NATS client protocol messages. It does
// no I/O.
//
// Parse only handles the control line. PUB payloads are binary and may contain
// CRLF, so the caller reads them separately using the size from the line.
package protocol

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Command is one of the *Cmd types below.
type Command interface{ isCommand() }

// ConnectCmd is `CONNECT {json}`.
type ConnectCmd struct {
	Options ConnectOptions
	Raw     []byte // the options JSON as sent
}

// PingCmd is `PING`.
type PingCmd struct{}

// PongCmd is `PONG`.
type PongCmd struct{}

// SubCmd is `SUB <subject> [queue] <sid>`.
type SubCmd struct {
	Subject string
	Queue   string
	Sid     string
}

// PubCmd is `PUB <subject> [reply] <#bytes>`. The payload follows separately.
type PubCmd struct {
	Subject string
	Reply   string
	Size    int
}

// UnsubCmd is `UNSUB <sid> [max_msgs]`.
type UnsubCmd struct {
	Sid     string
	MaxMsgs int
	HasMax  bool // whether max_msgs was given
}

func (ConnectCmd) isCommand() {}
func (PingCmd) isCommand()    {}
func (PongCmd) isCommand()    {}
func (SubCmd) isCommand()     {}
func (PubCmd) isCommand()     {}
func (UnsubCmd) isCommand()   {}

// ConnectOptions are the CONNECT options we read. Verbose defaults to true
// when absent, as in real NATS.
type ConnectOptions struct {
	Verbose  bool   `json:"verbose"`
	Pedantic bool   `json:"pedantic"`
	TLS      bool   `json:"tls_required"`
	Name     string `json:"name"`
	Lang     string `json:"lang"`
	Version  string `json:"version"`
}

var (
	// ErrEmptyLine is a blank line; the server ignores these.
	ErrEmptyLine   = errors.New("empty line")
	ErrUnknownOp   = errors.New("Unknown Protocol Operation")
	ErrInvalidArgs = errors.New("invalid arguments")
)

// Parse parses one control line. The CRLF is optional and verbs are
// case-insensitive.
func Parse(line []byte) (Command, error) {
	s := strings.TrimSpace(string(bytes.TrimRight(line, "\r\n")))
	if s == "" {
		return nil, ErrEmptyLine
	}

	verb, rest := splitVerb(s)
	switch strings.ToUpper(verb) {
	case "CONNECT":
		return parseConnect(rest)
	case "PING":
		return PingCmd{}, nil
	case "PONG":
		return PongCmd{}, nil
	case "SUB":
		return parseSub(rest)
	case "PUB":
		return parsePub(rest)
	case "UNSUB":
		return parseUnsub(rest)
	default:
		return nil, ErrUnknownOp
	}
}

// splitVerb splits off the first word. The rest is kept intact because the
// CONNECT JSON can contain spaces.
func splitVerb(s string) (verb, rest string) {
	i := strings.IndexAny(s, " \t")
	if i < 0 {
		return s, ""
	}
	return s[:i], strings.TrimSpace(s[i+1:])
}

func parseConnect(rest string) (Command, error) {
	opts := ConnectOptions{Verbose: true}
	raw := strings.TrimSpace(rest)
	if raw == "" {
		raw = "{}"
	}
	if err := json.Unmarshal([]byte(raw), &opts); err != nil {
		// Bad JSON is treated like "{}" rather than refusing the connection.
		return ConnectCmd{Options: ConnectOptions{Verbose: true}, Raw: []byte("{}")}, nil
	}
	return ConnectCmd{Options: opts, Raw: []byte(raw)}, nil
}

func parseSub(rest string) (Command, error) {
	f := strings.Fields(rest)
	switch len(f) {
	case 2:
		return SubCmd{Subject: f[0], Sid: f[1]}, nil
	case 3:
		return SubCmd{Subject: f[0], Queue: f[1], Sid: f[2]}, nil
	default:
		return nil, fmt.Errorf("%w: SUB requires <subject> [queue] <sid>", ErrInvalidArgs)
	}
}

func parsePub(rest string) (Command, error) {
	f := strings.Fields(rest)
	if len(f) != 2 && len(f) != 3 {
		return nil, fmt.Errorf("%w: PUB requires <subject> [reply] <#bytes>", ErrInvalidArgs)
	}
	size, err := strconv.Atoi(f[len(f)-1])
	if err != nil || size < 0 {
		return nil, fmt.Errorf("%w: PUB size", ErrInvalidArgs)
	}
	cmd := PubCmd{Subject: f[0], Size: size}
	if len(f) == 3 {
		cmd.Reply = f[1]
	}
	return cmd, nil
}

func parseUnsub(rest string) (Command, error) {
	f := strings.Fields(rest)
	switch len(f) {
	case 1:
		return UnsubCmd{Sid: f[0]}, nil
	case 2:
		max, err := strconv.Atoi(f[1])
		if err != nil || max < 0 {
			return nil, fmt.Errorf("%w: UNSUB max_msgs", ErrInvalidArgs)
		}
		return UnsubCmd{Sid: f[0], MaxMsgs: max, HasMax: true}, nil
	default:
		return nil, fmt.Errorf("%w: UNSUB requires <sid> [max_msgs]", ErrInvalidArgs)
	}
}
