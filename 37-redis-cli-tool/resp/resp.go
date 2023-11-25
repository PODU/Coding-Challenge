// Package resp encodes commands and decodes replies in Redis's RESP protocol.
package resp

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
)

type Type int

const (
	SimpleString Type = iota // +OK\r\n
	Error                    // -ERR ...\r\n
	Integer                  // :42\r\n
	BulkString               // $5\r\nhello\r\n  (IsNull for $-1)
	Array                    // *2\r\n...        (IsNull for *-1)
	Null
)

// maxBulkLen is Redis's own limit on a bulk string. Anything bigger is a
// corrupt or hostile length, and we don't want to allocate it.
const maxBulkLen = 512 * 1024 * 1024

type Value struct {
	Type   Type
	Str    string  // SimpleString, Error, BulkString
	Int    int64   // Integer
	Array  []Value // Array
	IsNull bool    // null bulk string ($-1) or null array (*-1)
}

// Encode turns a command into an array of bulk strings, e.g. SET name John
// becomes *3\r\n$3\r\nSET\r\n$4\r\nname\r\n$4\r\nJohn\r\n.
func Encode(args []string) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "*%d\r\n", len(args))
	for _, a := range args {
		fmt.Fprintf(&b, "$%d\r\n", len(a))
		b.WriteString(a)
		b.WriteString("\r\n")
	}
	return b.Bytes()
}

// Marshal encodes a reply. Only the fake server in the tests needs it.
func Marshal(v Value) []byte {
	var b bytes.Buffer
	marshal(&b, v)
	return b.Bytes()
}

func marshal(b *bytes.Buffer, v Value) {
	switch v.Type {
	case SimpleString:
		b.WriteByte('+')
		b.WriteString(v.Str)
		b.WriteString("\r\n")
	case Error:
		b.WriteByte('-')
		b.WriteString(v.Str)
		b.WriteString("\r\n")
	case Integer:
		b.WriteByte(':')
		b.WriteString(strconv.FormatInt(v.Int, 10))
		b.WriteString("\r\n")
	case BulkString:
		if v.IsNull {
			b.WriteString("$-1\r\n")
			return
		}
		fmt.Fprintf(b, "$%d\r\n", len(v.Str))
		b.WriteString(v.Str)
		b.WriteString("\r\n")
	case Array:
		if v.IsNull {
			b.WriteString("*-1\r\n")
			return
		}
		fmt.Fprintf(b, "*%d\r\n", len(v.Array))
		for _, e := range v.Array {
			marshal(b, e)
		}
	case Null:
		b.WriteString("$-1\r\n")
	}
}

// Decode reads one reply from r.
func Decode(r *bufio.Reader) (Value, error) {
	prefix, err := r.ReadByte()
	if err != nil {
		return Value{}, err
	}
	switch prefix {
	case '+':
		line, err := readLine(r)
		if err != nil {
			return Value{}, err
		}
		return Value{Type: SimpleString, Str: line}, nil
	case '-':
		line, err := readLine(r)
		if err != nil {
			return Value{}, err
		}
		return Value{Type: Error, Str: line}, nil
	case ':':
		n, err := readInt(r)
		if err != nil {
			return Value{}, err
		}
		return Value{Type: Integer, Int: n}, nil
	case '$':
		n, err := readInt(r)
		if err != nil {
			return Value{}, err
		}
		if n == -1 {
			return Value{Type: BulkString, IsNull: true}, nil
		}
		if n < 0 || n > maxBulkLen {
			return Value{}, fmt.Errorf("invalid bulk length %d", n)
		}
		buf := make([]byte, n)
		if _, err := io.ReadFull(r, buf); err != nil {
			return Value{}, err
		}
		if err := expectCRLF(r); err != nil {
			return Value{}, err
		}
		return Value{Type: BulkString, Str: string(buf)}, nil
	case '*':
		n, err := readInt(r)
		if err != nil {
			return Value{}, err
		}
		if n == -1 {
			return Value{Type: Array, IsNull: true}, nil
		}
		if n < 0 || n > maxBulkLen {
			return Value{}, fmt.Errorf("invalid array length %d", n)
		}
		arr := make([]Value, 0, n)
		for i := int64(0); i < n; i++ {
			el, err := Decode(r)
			if err != nil {
				return Value{}, err
			}
			arr = append(arr, el)
		}
		return Value{Type: Array, Array: arr}, nil
	default:
		return Value{}, fmt.Errorf("unknown RESP type byte %q", prefix)
	}
}

func readLine(r *bufio.Reader) (string, error) {
	s, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	if len(s) < 2 || s[len(s)-2] != '\r' {
		return "", errors.New("malformed RESP line: missing CRLF")
	}
	return s[:len(s)-2], nil
}

func readInt(r *bufio.Reader) (int64, error) {
	line, err := readLine(r)
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseInt(line, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid integer %q: %w", line, err)
	}
	return n, nil
}

func expectCRLF(r *bufio.Reader) error {
	cr, err := r.ReadByte()
	if err != nil {
		return err
	}
	lf, err := r.ReadByte()
	if err != nil {
		return err
	}
	if cr != '\r' || lf != '\n' {
		return errors.New("malformed RESP bulk string: missing trailing CRLF")
	}
	return nil
}
