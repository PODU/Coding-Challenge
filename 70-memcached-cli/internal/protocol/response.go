package protocol

import (
	"bufio"
	"io"
	"strconv"
	"strings"
)

// StoreStatus is the outcome of a storage command.
type StoreStatus int

const (
	Stored StoreStatus = iota
	// NotStored: the condition failed, e.g. add on a key that exists.
	NotStored
	// Exists: cas failed because the item changed.
	Exists
	// NotFound: cas on a missing item.
	NotFound
	// NoReply: sent with noreply, so there is no answer.
	NoReply
)

func (s StoreStatus) String() string {
	switch s {
	case Stored:
		return "STORED"
	case NotStored:
		return "NOT_STORED"
	case Exists:
		return "EXISTS"
	case NotFound:
		return "NOT_FOUND"
	case NoReply:
		return "(noreply)"
	default:
		return "UNKNOWN"
	}
}

// Value is one item from a get or gets reply.
type Value struct {
	Key    string
	Flags  uint32
	CAS    uint64
	HasCAS bool
	Data   []byte
}

// Stat is one "STAT <name> <value>" line from a stats reply.
type Stat struct {
	Name  string
	Value string
}

// readLine returns the next line without its line ending. A bare \n is
// accepted too.
func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		if err == io.EOF && line != "" {
			return strings.TrimRight(line, "\r\n"), io.ErrUnexpectedEOF
		}
		return "", err
	}
	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")
	return line, nil
}

// ParseStorageReply parses the reply to a storage command.
func ParseStorageReply(r *bufio.Reader) (StoreStatus, error) {
	line, err := readLine(r)
	if err != nil {
		return 0, err
	}
	switch line {
	case "STORED":
		return Stored, nil
	case "NOT_STORED":
		return NotStored, nil
	case "EXISTS":
		return Exists, nil
	case "NOT_FOUND":
		return NotFound, nil
	}
	if perr := mapErrorLine(line); perr != nil {
		return 0, perr
	}
	return 0, ErrUnexpectedReply
}

// ParseValues parses VALUE blocks up to END. Data is read by its declared
// length, since it may itself contain CRLF.
func ParseValues(r *bufio.Reader) ([]Value, error) {
	var values []Value
	for {
		line, err := readLine(r)
		if err != nil {
			return nil, err
		}
		if line == "END" {
			return values, nil
		}
		if !strings.HasPrefix(line, "VALUE ") {
			if perr := mapErrorLine(line); perr != nil {
				return nil, perr
			}
			return nil, ErrUnexpectedReply
		}
		v, err := parseValueHeader(line)
		if err != nil {
			return nil, err
		}
		buf := make([]byte, v.byteCount)
		if _, err := io.ReadFull(r, buf); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil, ErrTruncated
			}
			return nil, err
		}
		if err := consumeCRLF(r); err != nil {
			return nil, err
		}
		v.value.Data = buf
		values = append(values, v.value)
	}
}

type valueHeader struct {
	value     Value
	byteCount int // length of the data that follows
}

// parseValueHeader parses "VALUE <key> <flags> <bytes> [<cas unique>]".
func parseValueHeader(line string) (valueHeader, error) {
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return valueHeader{}, ErrUnexpectedReply
	}
	var vh valueHeader
	vh.value.Key = fields[1]
	flags, err := strconv.ParseUint(fields[2], 10, 32)
	if err != nil {
		return valueHeader{}, ErrUnexpectedReply
	}
	vh.value.Flags = uint32(flags)
	n, err := strconv.Atoi(fields[3])
	if err != nil || n < 0 {
		return valueHeader{}, ErrUnexpectedReply
	}
	vh.byteCount = n
	if len(fields) >= 5 {
		cas, err := strconv.ParseUint(fields[4], 10, 64)
		if err != nil {
			return valueHeader{}, ErrUnexpectedReply
		}
		vh.value.CAS = cas
		vh.value.HasCAS = true
	}
	return vh, nil
}

// consumeCRLF reads the \r\n that ends a data block.
func consumeCRLF(r *bufio.Reader) error {
	crlf := make([]byte, 2)
	if _, err := io.ReadFull(r, crlf); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return ErrTruncated
		}
		return err
	}
	if crlf[0] != '\r' || crlf[1] != '\n' {
		return ErrUnexpectedReply
	}
	return nil
}

// ParseDeleteReply returns true for DELETED and false for NOT_FOUND.
func ParseDeleteReply(r *bufio.Reader) (bool, error) {
	line, err := readLine(r)
	if err != nil {
		return false, err
	}
	switch line {
	case "DELETED":
		return true, nil
	case "NOT_FOUND":
		return false, nil
	}
	if perr := mapErrorLine(line); perr != nil {
		return false, perr
	}
	return false, ErrUnexpectedReply
}

// ParseNumericReply parses an incr or decr reply. found is false for
// NOT_FOUND.
func ParseNumericReply(r *bufio.Reader) (value uint64, found bool, err error) {
	line, err := readLine(r)
	if err != nil {
		return 0, false, err
	}
	if line == "NOT_FOUND" {
		return 0, false, nil
	}
	if perr := mapErrorLine(line); perr != nil {
		return 0, false, perr
	}
	n, perr := strconv.ParseUint(line, 10, 64)
	if perr != nil {
		return 0, false, ErrUnexpectedReply
	}
	return n, true, nil
}

// ParseStats parses "STAT <name> <value>" lines up to END.
func ParseStats(r *bufio.Reader) ([]Stat, error) {
	var stats []Stat
	for {
		line, err := readLine(r)
		if err != nil {
			return nil, err
		}
		if line == "END" {
			return stats, nil
		}
		if !strings.HasPrefix(line, "STAT ") {
			if perr := mapErrorLine(line); perr != nil {
				return nil, perr
			}
			return nil, ErrUnexpectedReply
		}
		name, value, _ := strings.Cut(strings.TrimPrefix(line, "STAT "), " ")
		stats = append(stats, Stat{Name: name, Value: value})
	}
}
