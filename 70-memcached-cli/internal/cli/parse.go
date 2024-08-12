// Package cli parses ccmc commands (from argv or the REPL), runs them and
// formats the replies.
package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/codingchallenges/ccmc/internal/protocol"
)

// Globals are the global flags that commands pick up.
type Globals struct {
	Flags   uint32
	Exptime int64
	NoReply bool
	Quiet   bool
}

// Command is a parsed and validated command.
type Command struct {
	Name    string
	Keys    []string
	Flags   uint32
	Exptime int64
	Data    []byte
	CAS     uint64
	HasCAS  bool
	Delta   uint64
	NoReply bool
	Quiet   bool
}

// UsageError is a bad command line (exit code 2).
type UsageError struct{ Msg string }

func (e *UsageError) Error() string { return e.Msg }

func usagef(format string, a ...any) error { return &UsageError{Msg: fmt.Sprintf(format, a...)} }

var storageCommands = map[string]bool{
	"set": true, "add": true, "replace": true, "append": true, "prepend": true, "cas": true,
}

// ParseCommand parses a command and its arguments. A storage command's value
// is the remaining arguments joined with spaces; if there are none it is read
// from stdin, which lets you pipe in binary data. The REPL passes a nil stdin
// to turn that off.
func ParseCommand(args []string, g Globals, stdin io.Reader) (*Command, error) {
	if len(args) == 0 {
		return nil, usagef("no command given")
	}
	name := strings.ToLower(args[0])
	rest := args[1:]

	cmd := &Command{
		Name:    name,
		Flags:   g.Flags,
		Exptime: g.Exptime,
		NoReply: g.NoReply,
		Quiet:   g.Quiet,
	}

	switch name {
	case "set", "add", "replace", "append", "prepend":
		if len(rest) < 1 {
			return nil, usagef("%s requires a key", name)
		}
		if err := validateKey(rest[0]); err != nil {
			return nil, err
		}
		cmd.Keys = []string{rest[0]}
		data, err := readData(rest[1:], stdin)
		if err != nil {
			return nil, err
		}
		cmd.Data = data

	case "cas":
		if len(rest) < 2 {
			return nil, usagef("cas requires a key and a cas id")
		}
		if err := validateKey(rest[0]); err != nil {
			return nil, err
		}
		casid, err := strconv.ParseUint(rest[1], 10, 64)
		if err != nil {
			return nil, usagef("cas id must be a non-negative integer: %q", rest[1])
		}
		cmd.Keys = []string{rest[0]}
		cmd.CAS = casid
		cmd.HasCAS = true
		data, err := readData(rest[2:], stdin)
		if err != nil {
			return nil, err
		}
		cmd.Data = data

	case "get", "gets":
		if len(rest) < 1 {
			return nil, usagef("%s requires at least one key", name)
		}
		for _, k := range rest {
			if err := validateKey(k); err != nil {
				return nil, err
			}
		}
		cmd.Keys = append([]string(nil), rest...)

	case "delete":
		if len(rest) != 1 {
			return nil, usagef("delete requires exactly one key")
		}
		if err := validateKey(rest[0]); err != nil {
			return nil, err
		}
		cmd.Keys = []string{rest[0]}

	case "incr", "decr":
		if len(rest) != 2 {
			return nil, usagef("%s requires a key and a delta", name)
		}
		if err := validateKey(rest[0]); err != nil {
			return nil, err
		}
		delta, err := strconv.ParseUint(rest[1], 10, 64)
		if err != nil {
			return nil, usagef("%s delta must be a non-negative integer: %q", name, rest[1])
		}
		cmd.Keys = []string{rest[0]}
		cmd.Delta = delta

	case "stats":
		if len(rest) > 1 {
			return nil, usagef("stats takes at most one argument")
		}
		if len(rest) == 1 {
			cmd.Keys = []string{rest[0]}
		}

	default:
		return nil, usagef("unknown command %q", name)
	}

	return cmd, nil
}

// readData returns a storage command's value, from args or else stdin.
func readData(args []string, stdin io.Reader) ([]byte, error) {
	if len(args) > 0 {
		return []byte(strings.Join(args, " ")), nil
	}
	if stdin == nil {
		return []byte{}, nil
	}
	data, err := io.ReadAll(stdin)
	if err != nil {
		return nil, usagef("reading value from stdin: %v", err)
	}
	return data, nil
}

// validateKey checks the protocol's key rules: 1 to 250 bytes, with no
// control characters or spaces.
func validateKey(key string) error {
	if len(key) == 0 {
		return usagef("key must not be empty")
	}
	if len(key) > 250 {
		return usagef("key %q exceeds 250 bytes", key)
	}
	for i := 0; i < len(key); i++ {
		c := key[i]
		if c <= 0x20 || c == 0x7f {
			return usagef("key %q contains an invalid character (control or space)", key)
		}
	}
	return nil
}

// IsStorage reports whether a command name takes a data block.
func IsStorage(name string) bool { return storageCommands[name] }

// StorageCommand returns c as a protocol storage request.
func (c *Command) StorageCommand() protocol.StorageCommand {
	return protocol.StorageCommand{
		Command: c.Name,
		Key:     c.Keys[0],
		Flags:   c.Flags,
		Exptime: c.Exptime,
		Data:    c.Data,
		CAS:     c.CAS,
		HasCAS:  c.HasCAS,
		NoReply: c.NoReply,
	}
}
