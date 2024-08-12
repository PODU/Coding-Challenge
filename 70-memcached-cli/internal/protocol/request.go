// Package protocol encodes requests and parses replies for the memcached text
// protocol. It does no networking itself.
//
// Reference: https://github.com/memcached/memcached/blob/master/doc/protocol.txt
package protocol

import (
	"bytes"
	"strconv"
)

// StorageCommand is a set, add, replace, append, prepend or cas request.
type StorageCommand struct {
	Command string
	Key     string
	Flags   uint32
	Exptime int64
	Data    []byte
	CAS     uint64 // only sent for cas
	HasCAS  bool
	NoReply bool
}

// EncodeStorage builds the wire bytes for a storage command:
//
//	<command> <key> <flags> <exptime> <bytes> [<cas unique>] [noreply]\r\n
//	<data block of exactly <bytes> bytes>\r\n
//
// Data may contain CRLF or NUL; the length prefix is what frames it.
func EncodeStorage(c StorageCommand) []byte {
	var b bytes.Buffer
	b.WriteString(c.Command)
	b.WriteByte(' ')
	b.WriteString(c.Key)
	b.WriteByte(' ')
	b.WriteString(strconv.FormatUint(uint64(c.Flags), 10))
	b.WriteByte(' ')
	b.WriteString(strconv.FormatInt(c.Exptime, 10))
	b.WriteByte(' ')
	b.WriteString(strconv.Itoa(len(c.Data)))
	if c.HasCAS || c.Command == "cas" {
		b.WriteByte(' ')
		b.WriteString(strconv.FormatUint(c.CAS, 10))
	}
	if c.NoReply {
		b.WriteString(" noreply")
	}
	b.WriteString("\r\n")
	b.Write(c.Data)
	b.WriteString("\r\n")
	return b.Bytes()
}

// EncodeRetrieval builds a "get"/"gets" request for one or more keys:
//
//	<command> <key>*\r\n
func EncodeRetrieval(command string, keys []string) []byte {
	var b bytes.Buffer
	b.WriteString(command)
	for _, k := range keys {
		b.WriteByte(' ')
		b.WriteString(k)
	}
	b.WriteString("\r\n")
	return b.Bytes()
}

// EncodeDelete builds a "delete <key> [noreply]\r\n" request.
func EncodeDelete(key string, noreply bool) []byte {
	var b bytes.Buffer
	b.WriteString("delete ")
	b.WriteString(key)
	if noreply {
		b.WriteString(" noreply")
	}
	b.WriteString("\r\n")
	return b.Bytes()
}

// EncodeIncrDecr builds an "incr|decr <key> <delta> [noreply]\r\n" request.
func EncodeIncrDecr(command, key string, delta uint64, noreply bool) []byte {
	var b bytes.Buffer
	b.WriteString(command)
	b.WriteByte(' ')
	b.WriteString(key)
	b.WriteByte(' ')
	b.WriteString(strconv.FormatUint(delta, 10))
	if noreply {
		b.WriteString(" noreply")
	}
	b.WriteString("\r\n")
	return b.Bytes()
}

// EncodeStats builds a "stats\r\n" request, or "stats <arg>\r\n" when arg != "".
func EncodeStats(arg string) []byte {
	var b bytes.Buffer
	b.WriteString("stats")
	if arg != "" {
		b.WriteByte(' ')
		b.WriteString(arg)
	}
	b.WriteString("\r\n")
	return b.Bytes()
}
