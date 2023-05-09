package server

import (
	"fmt"
	"strings"

	"rediserver/internal/resp"
	"rediserver/internal/store"
)

// handler runs one command; args excludes the command name. Handlers return a
// reply and never touch the socket.
type handler func(st *store.Store, args []string) resp.Value

var commands = map[string]handler{
	"PING":    cmdPing,
	"ECHO":    cmdEcho,
	"SET":     cmdSet,
	"GET":     cmdGet,
	"EXISTS":  cmdExists,
	"DEL":     cmdDel,
	"INCR":    cmdIncr,
	"DECR":    cmdDecr,
	"LPUSH":   cmdLPush,
	"RPUSH":   cmdRPush,
	"LRANGE":  cmdLRange,
	"COMMAND": cmdCommand,
}

// dispatch runs the command in v. The bool is false when there is nothing to
// reply to (not an array, or an empty one).
func (s *Server) dispatch(v resp.Value) (resp.Value, bool) {
	if v.Kind != resp.Array || v.IsNull || len(v.Arr) == 0 {
		return resp.Value{}, false
	}

	args := make([]string, len(v.Arr))
	for i, e := range v.Arr {
		args[i] = e.Str
	}

	name := strings.ToUpper(args[0])
	rest := args[1:]

	// SAVE needs the server's dbfile path, so it is not in the table.
	if name == "SAVE" {
		return s.cmdSave(rest), true
	}

	h, ok := commands[name]
	if !ok {
		return errUnknown(args[0]), true
	}
	return h(s.Store, rest), true
}

// These error strings match real Redis byte for byte; clients match on them.

func errWrongArgs(name string) resp.Value {
	return resp.ErrorString(fmt.Sprintf("ERR wrong number of arguments for '%s' command", strings.ToLower(name)))
}

func errUnknown(name string) resp.Value {
	return resp.ErrorString(fmt.Sprintf("ERR unknown command '%s'", name))
}

func errNotInt() resp.Value {
	return resp.ErrorString("ERR value is not an integer or out of range")
}

func errWrongType() resp.Value {
	return resp.ErrorString("WRONGTYPE Operation against a key holding the wrong kind of value")
}

func errSyntax() resp.Value {
	return resp.ErrorString("ERR syntax error")
}
