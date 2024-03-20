package bloom

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
)

// File format, integers big-endian:
//
//	offset  size  field
//	0       4     magic    = "CCBF"
//	4       2     version  = uint16 (currently 1)
//	6       2     k        = uint16 (number of hash functions)
//	8       4     m        = uint32 (number of bits)
//	12      ...   bits     = ceil(m/8) raw bitset bytes
const (
	headerSize    = 12
	formatVersion = uint16(1)
)

// Magic starts every model file.
var Magic = [4]byte{'C', 'C', 'B', 'F'}

// ParamError means the false-positive rate was outside (0, 1).
type ParamError struct {
	P float64
}

func (e *ParamError) Error() string {
	return fmt.Sprintf("bloom: target false-positive probability must be in (0, 1), got %g", e.P)
}

// FormatError means a model file is malformed or from an unknown version.
type FormatError struct {
	Reason string
}

func (e *FormatError) Error() string { return "bloom: " + e.Reason }

// WriteTo writes the filter in the CCBF format.
func (bf *BloomFilter) WriteTo(w io.Writer) (int64, error) {
	bw := bufio.NewWriter(w)

	var header [headerSize]byte
	copy(header[0:4], Magic[:])
	binary.BigEndian.PutUint16(header[4:6], formatVersion)
	binary.BigEndian.PutUint16(header[6:8], bf.k)
	binary.BigEndian.PutUint32(header[8:12], bf.m)

	var total int64
	n, err := bw.Write(header[:])
	total += int64(n)
	if err != nil {
		return total, err
	}

	n, err = bw.Write(bf.bits)
	total += int64(n)
	if err != nil {
		return total, err
	}

	return total, bw.Flush()
}

// ReadFrom replaces bf with a filter read in the CCBF format. Problems with
// the file are reported as *FormatError.
func (bf *BloomFilter) ReadFrom(r io.Reader) (int64, error) {
	br := bufio.NewReader(r)

	var header [headerSize]byte
	hn, err := io.ReadFull(br, header[:])
	total := int64(hn)
	if err != nil {
		return total, &FormatError{Reason: "truncated header: " + err.Error()}
	}

	if [4]byte(header[0:4]) != Magic {
		return total, &FormatError{Reason: fmt.Sprintf("bad magic: expected %q, got %q", string(Magic[:]), string(header[0:4]))}
	}

	version := binary.BigEndian.Uint16(header[4:6])
	if version != formatVersion {
		return total, &FormatError{Reason: fmt.Sprintf("unsupported version %d (supported: %d)", version, formatVersion)}
	}

	k := binary.BigEndian.Uint16(header[6:8])
	m := binary.BigEndian.Uint32(header[8:12])
	if k < 1 {
		return total, &FormatError{Reason: "invalid header: k must be at least 1"}
	}
	if m < 1 {
		return total, &FormatError{Reason: "invalid header: m must be at least 1"}
	}

	nbytes := (int(m) + 7) / 8
	bits := make([]byte, nbytes)
	bn, err := io.ReadFull(br, bits)
	total += int64(bn)
	if err != nil {
		return total, &FormatError{Reason: fmt.Sprintf("truncated bit array: expected %d bytes, %v", nbytes, err)}
	}

	bf.m = m
	bf.k = k
	bf.bits = bits
	return total, nil
}
