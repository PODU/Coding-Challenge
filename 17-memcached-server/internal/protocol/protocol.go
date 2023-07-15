// Package protocol parses memcached text-protocol command lines and holds the
// response strings.
//
// It only deals with the command line itself. Reading the data block that
// follows a storage command is left to the caller, which owns the connection.
package protocol

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	RespStored    = "STORED\r\n"
	RespNotStored = "NOT_STORED\r\n"
	RespExists    = "EXISTS\r\n"
	RespNotFound  = "NOT_FOUND\r\n"
	RespDeleted   = "DELETED\r\n"
	RespEnd       = "END\r\n"
	RespError     = "ERROR\r\n"
)

// MaxKeyLen is memcached's key length limit in bytes.
const MaxKeyLen = 250

// ClientError formats a CLIENT_ERROR response line.
func ClientError(msg string) string { return "CLIENT_ERROR " + msg + "\r\n" }

// ServerError formats a SERVER_ERROR response line.
func ServerError(msg string) string { return "SERVER_ERROR " + msg + "\r\n" }

// Type is the kind of command.
type Type int

const (
	// TypeStorage covers set/add/replace/append/prepend/cas.
	TypeStorage Type = iota
	// TypeRetrieval covers get/gets.
	TypeRetrieval
	// TypeDelete covers delete.
	TypeDelete
)

// Command is a parsed request line. For storage commands the caller reads the
// Bytes-long data block that follows.
type Command struct {
	Type      Type
	Op        string
	Key       string   // storage and delete
	Keys      []string // get and gets
	Flags     uint32
	Exptime   int64
	Bytes     int    // declared data-block length, excluding trailing CRLF
	CASUnique uint64 // cas only
	NoReply   bool
}

// ParseError distinguishes an unknown command (-> ERROR) from a malformed but
// recognised command (-> CLIENT_ERROR <msg>).
type ParseError struct {
	Client bool
	Msg    string
}

func (e *ParseError) Error() string { return e.Msg }

func badLine() *ParseError { return &ParseError{Client: true, Msg: "bad command line format"} }
func unknown() *ParseError { return &ParseError{Client: false, Msg: "ERROR"} }

// Parse parses one command line, with or without its trailing CRLF.
func Parse(line string) (*Command, *ParseError) {
	line = strings.TrimRight(line, "\r\n")
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil, unknown()
	}
	op := fields[0]
	switch op {
	case "set", "add", "replace", "append", "prepend", "cas":
		return parseStorage(op, fields)
	case "get", "gets":
		return parseRetrieval(op, fields)
	case "delete":
		return parseDelete(fields)
	default:
		return nil, unknown()
	}
}

func parseStorage(op string, f []string) (*Command, *ParseError) {
	nargs := 5
	if op == "cas" {
		nargs = 6
	}
	if len(f) < nargs {
		return nil, badLine()
	}

	cmd := &Command{Type: TypeStorage, Op: op, Key: f[1]}
	if len(cmd.Key) == 0 || len(cmd.Key) > MaxKeyLen {
		return nil, badLine()
	}

	flags, err := strconv.ParseUint(f[2], 10, 32)
	if err != nil {
		return nil, badLine()
	}
	cmd.Flags = uint32(flags)

	exp, err := strconv.ParseInt(f[3], 10, 64)
	if err != nil {
		return nil, badLine()
	}
	cmd.Exptime = exp

	nbytes, err := strconv.Atoi(f[4])
	if err != nil || nbytes < 0 {
		return nil, badLine()
	}
	cmd.Bytes = nbytes

	if op == "cas" {
		cu, err := strconv.ParseUint(f[5], 10, 64)
		if err != nil {
			return nil, badLine()
		}
		cmd.CASUnique = cu
	}

	if len(f) > nargs {
		if f[nargs] != "noreply" {
			return nil, badLine()
		}
		cmd.NoReply = true
	}
	if len(f) > nargs+1 {
		return nil, badLine()
	}
	return cmd, nil
}

func parseRetrieval(op string, f []string) (*Command, *ParseError) {
	if len(f) < 2 {
		return nil, badLine()
	}
	keys := f[1:]
	for _, k := range keys {
		if len(k) == 0 || len(k) > MaxKeyLen {
			return nil, badLine()
		}
	}
	return &Command{Type: TypeRetrieval, Op: op, Keys: keys}, nil
}

func parseDelete(f []string) (*Command, *ParseError) {
	if len(f) < 2 || len(f[1]) > MaxKeyLen {
		return nil, badLine()
	}
	cmd := &Command{Type: TypeDelete, Key: f[1]}
	if len(f) > 2 {
		if f[2] != "noreply" {
			return nil, badLine()
		}
		cmd.NoReply = true
	}
	if len(f) > 3 {
		return nil, badLine()
	}
	return cmd, nil
}

// Encode is the inverse of Parse for well-formed commands.
func (c *Command) Encode() string {
	switch c.Type {
	case TypeStorage:
		var b strings.Builder
		fmt.Fprintf(&b, "%s %s %d %d %d", c.Op, c.Key, c.Flags, c.Exptime, c.Bytes)
		if c.Op == "cas" {
			fmt.Fprintf(&b, " %d", c.CASUnique)
		}
		if c.NoReply {
			b.WriteString(" noreply")
		}
		b.WriteString("\r\n")
		return b.String()
	case TypeRetrieval:
		return c.Op + " " + strings.Join(c.Keys, " ") + "\r\n"
	case TypeDelete:
		s := "delete " + c.Key
		if c.NoReply {
			s += " noreply"
		}
		return s + "\r\n"
	}
	return ""
}
