package protocol

import (
	"errors"
	"strings"
)

// ProtocolError is an ERROR, CLIENT_ERROR or SERVER_ERROR reply.
type ProtocolError struct {
	Kind string // "ERROR", "CLIENT_ERROR" or "SERVER_ERROR"
	Msg  string // empty for ERROR
}

func (e *ProtocolError) Error() string {
	if e.Msg == "" {
		return e.Kind
	}
	return e.Kind + ": " + e.Msg
}

// IsClientError reports whether the error is a CLIENT_ERROR reply.
func (e *ProtocolError) IsClientError() bool { return e.Kind == "CLIENT_ERROR" }

// IsServerError reports whether the error is a SERVER_ERROR reply.
func (e *ProtocolError) IsServerError() bool { return e.Kind == "SERVER_ERROR" }

// ErrUnexpectedReply means the reply made no sense for the command sent.
var ErrUnexpectedReply = errors.New("protocol: unexpected reply from server")

// ErrTruncated means the connection ended in the middle of a value.
var ErrTruncated = errors.New("protocol: truncated value data block")

// mapErrorLine returns a *ProtocolError if line is an error reply, else nil.
func mapErrorLine(line string) error {
	switch {
	case line == "ERROR":
		return &ProtocolError{Kind: "ERROR"}
	case strings.HasPrefix(line, "CLIENT_ERROR"):
		return &ProtocolError{Kind: "CLIENT_ERROR", Msg: strings.TrimSpace(strings.TrimPrefix(line, "CLIENT_ERROR"))}
	case strings.HasPrefix(line, "SERVER_ERROR"):
		return &ProtocolError{Kind: "SERVER_ERROR", Msg: strings.TrimSpace(strings.TrimPrefix(line, "SERVER_ERROR"))}
	default:
		return nil
	}
}
