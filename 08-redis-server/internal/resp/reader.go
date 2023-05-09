package resp

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"strings"
)

// MaxBulkLen is the largest bulk string we accept (512MB, same as Redis), so a
// bad length prefix can't make us allocate unbounded memory.
const MaxBulkLen = 512 * 1024 * 1024

// ProtocolError means the client sent malformed RESP. The connection handler
// replies with an error and closes the connection.
type ProtocolError struct{ msg string }

func (e *ProtocolError) Error() string { return e.msg }

func protoErr(msg string) error { return &ProtocolError{msg: msg} }

// IsProtocolError reports whether err is a RESP protocol error.
func IsProtocolError(err error) bool {
	var pe *ProtocolError
	return errors.As(err, &pe)
}

// Reader decodes RESP values from a buffered byte stream.
type Reader struct {
	r *bufio.Reader
}

// NewReader returns a Reader that decodes from r.
func NewReader(r *bufio.Reader) *Reader { return &Reader{r: r} }

// Read decodes one RESP value, returning io.EOF if the stream ends cleanly
// between values. Lines that don't start with a RESP prefix byte are parsed as
// inline commands so that plain telnet sessions work.
func (rd *Reader) Read() (Value, error) {
	b, err := rd.r.ReadByte()
	if err != nil {
		return Value{}, err
	}
	switch Kind(b) {
	case Simple:
		s, err := rd.readLine()
		return Value{Kind: Simple, Str: s}, err
	case Error:
		s, err := rd.readLine()
		return Value{Kind: Error, Str: s}, err
	case Integer:
		return rd.readInteger()
	case Bulk:
		return rd.readBulk()
	case Array:
		return rd.readArray()
	default:
		return rd.readInline(b)
	}
}

// readLine returns the next line without its CRLF.
func (rd *Reader) readLine() (string, error) {
	line, err := rd.r.ReadString('\n')
	if err == io.EOF && len(line) > 0 {
		return "", io.ErrUnexpectedEOF
	}
	if err != nil {
		return "", err
	}
	if len(line) < 2 || line[len(line)-2] != '\r' {
		return "", protoErr("expected CRLF line terminator")
	}
	return line[:len(line)-2], nil
}

func (rd *Reader) readInteger() (Value, error) {
	line, err := rd.readLine()
	if err != nil {
		return Value{}, err
	}
	n, err := strconv.ParseInt(line, 10, 64)
	if err != nil {
		return Value{}, protoErr("invalid integer")
	}
	return Value{Kind: Integer, Int: n}, nil
}

func (rd *Reader) readBulk() (Value, error) {
	line, err := rd.readLine()
	if err != nil {
		return Value{}, err
	}
	n, err := strconv.Atoi(line)
	if err != nil {
		return Value{}, protoErr("invalid bulk length")
	}
	if n == -1 {
		return Value{Kind: Bulk, IsNull: true}, nil
	}
	if n < 0 {
		return Value{}, protoErr("invalid bulk length")
	}
	if n > MaxBulkLen {
		return Value{}, protoErr("bulk string exceeds maximum length")
	}
	// Trust the declared length: the payload is binary-safe and may itself
	// contain CRLF.
	buf := make([]byte, n+2)
	if _, err := io.ReadFull(rd.r, buf); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return Value{}, io.ErrUnexpectedEOF
		}
		return Value{}, err
	}
	if buf[n] != '\r' || buf[n+1] != '\n' {
		return Value{}, protoErr("expected CRLF after bulk string")
	}
	return Value{Kind: Bulk, Str: string(buf[:n])}, nil
}

func (rd *Reader) readArray() (Value, error) {
	line, err := rd.readLine()
	if err != nil {
		return Value{}, err
	}
	n, err := strconv.Atoi(line)
	if err != nil {
		return Value{}, protoErr("invalid array length")
	}
	if n == -1 {
		return Value{Kind: Array, IsNull: true}, nil
	}
	if n < 0 {
		return Value{}, protoErr("invalid array length")
	}
	elems := make([]Value, 0, n)
	for i := 0; i < n; i++ {
		v, err := rd.Read()
		if err != nil {
			if err == io.EOF {
				return Value{}, io.ErrUnexpectedEOF
			}
			return Value{}, err
		}
		elems = append(elems, v)
	}
	return Value{Kind: Array, Arr: elems}, nil
}

// readInline splits a plain text line on whitespace. first is the byte Read
// already consumed.
func (rd *Reader) readInline(first byte) (Value, error) {
	rest, err := rd.r.ReadString('\n')
	if err != nil && err != io.EOF {
		return Value{}, err
	}
	line := strings.TrimRight(string(first)+rest, "\r\n")
	fields := strings.Fields(line)
	if len(fields) == 0 {
		if err == io.EOF {
			return Value{}, io.EOF
		}
		return Value{Kind: Array, Arr: []Value{}}, nil
	}
	elems := make([]Value, len(fields))
	for i, f := range fields {
		elems[i] = Value{Kind: Bulk, Str: f}
	}
	return Value{Kind: Array, Arr: elems}, nil
}
