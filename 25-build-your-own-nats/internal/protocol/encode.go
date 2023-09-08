package protocol

import (
	"encoding/json"
	"strconv"
)

const crlf = "\r\n"

// Info is the server INFO payload sent once on connect.
type Info struct {
	ServerID     string `json:"server_id"`
	ServerName   string `json:"server_name"`
	Version      string `json:"version"`
	Proto        int    `json:"proto"`
	Go           string `json:"go"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	MaxPayload   int    `json:"max_payload"`
	Headers      bool   `json:"headers"`
	ClientID     uint64 `json:"client_id"`
	AuthRequired bool   `json:"auth_required"`
	TLSRequired  bool   `json:"tls_required"`
}

// EncodeInfo renders `INFO {json}\r\n`.
func EncodeInfo(info Info) []byte {
	b, _ := json.Marshal(info)
	out := make([]byte, 0, len(b)+8)
	out = append(out, "INFO "...)
	out = append(out, b...)
	out = append(out, crlf...)
	return out
}

// OK renders `+OK\r\n`.
func OK() []byte { return []byte("+OK" + crlf) }

// Pong renders `PONG\r\n`.
func Pong() []byte { return []byte("PONG" + crlf) }

// Ping renders `PING\r\n`.
func Ping() []byte { return []byte("PING" + crlf) }

// EncodeErr renders `-ERR '<reason>'\r\n`.
func EncodeErr(reason string) []byte {
	return []byte("-ERR '" + reason + "'" + crlf)
}

// EncodeMsg renders `MSG <subject> <sid> [reply] <#bytes>\r\n<payload>\r\n`.
// An empty reply is left out.
func EncodeMsg(subject, sid, reply string, payload []byte) []byte {
	n := strconv.Itoa(len(payload))
	size := 4 + len(subject) + 1 + len(sid) + 1 + len(n) + 2 + len(payload) + 2
	if reply != "" {
		size += len(reply) + 1
	}
	out := make([]byte, 0, size)
	out = append(out, "MSG "...)
	out = append(out, subject...)
	out = append(out, ' ')
	out = append(out, sid...)
	out = append(out, ' ')
	if reply != "" {
		out = append(out, reply...)
		out = append(out, ' ')
	}
	out = append(out, n...)
	out = append(out, crlf...)
	out = append(out, payload...)
	out = append(out, crlf...)
	return out
}
