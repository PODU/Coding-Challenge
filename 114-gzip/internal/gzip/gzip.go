// Package gzip reads and writes the gzip container (RFC 1952) around a
// DEFLATE stream.
package gzip

import (
	"bufio"
	"errors"
	"fmt"
	"io"

	"gzipcc/internal/bitio"
	"gzipcc/internal/crc32"
	"gzipcc/internal/deflate"
)

// FLG bits.
const (
	flagText    = 1 << 0
	flagHCRC    = 1 << 1
	flagExtra   = 1 << 2
	flagName    = 1 << 3
	flagComment = 1 << 4
)

const (
	cmDeflate = 8
	osUnknown = 255
)

// Header is the metadata gzipcc reads from and writes to a member header.
type Header struct {
	Name    string
	ModTime uint32 // Unix seconds
}

// Compress writes src to w as a single gzip member.
func Compress(w io.Writer, src []byte, level int, hdr Header) error {
	xfl := byte(0)
	switch {
	case level >= 9:
		xfl = 2
	case level == 1:
		xfl = 4
	}
	flg := byte(0)
	if hdr.Name != "" {
		flg |= flagName
	}

	head := []byte{
		0x1f, 0x8b, cmDeflate, flg,
		byte(hdr.ModTime), byte(hdr.ModTime >> 8), byte(hdr.ModTime >> 16), byte(hdr.ModTime >> 24),
		xfl, osUnknown,
	}
	if _, err := w.Write(head); err != nil {
		return err
	}
	if hdr.Name != "" {
		if _, err := io.WriteString(w, hdr.Name); err != nil {
			return err
		}
		if _, err := w.Write([]byte{0}); err != nil {
			return err
		}
	}

	if err := deflate.Compress(w, src, level); err != nil {
		return err
	}

	crc := crc32.Checksum(src)
	isize := uint32(len(src)) // size mod 2^32
	tail := []byte{
		byte(crc), byte(crc >> 8), byte(crc >> 16), byte(crc >> 24),
		byte(isize), byte(isize >> 8), byte(isize >> 16), byte(isize >> 24),
	}
	_, err := w.Write(tail)
	return err
}

var (
	ErrChecksum = errors.New("gzip: CRC-32 checksum mismatch")
	ErrSize     = errors.New("gzip: ISIZE length mismatch")
	ErrHeader   = errors.New("gzip: invalid header")
)

// sink buffers inflated output, writing it to dst (and into the CRC) once it
// is older than the 32 KiB that back-references can reach.
type sink struct {
	dst   io.Writer
	buf   []byte
	crc   uint32
	total int64
}

func newSink(dst io.Writer) *sink {
	return &sink{dst: dst, buf: make([]byte, 0, 1<<17)}
}

func (s *sink) WriteByte(b byte) error {
	s.buf = append(s.buf, b)
	s.total++
	return s.maybeFlush()
}

func (s *sink) CopyMatch(dist, length int) error {
	start := len(s.buf) - dist
	for i := 0; i < length; i++ {
		s.buf = append(s.buf, s.buf[start+i])
	}
	s.total += int64(length)
	return s.maybeFlush()
}

func (s *sink) Len() int64 { return s.total }

func (s *sink) maybeFlush() error {
	if len(s.buf) <= 1<<16 {
		return nil
	}
	return s.flushN(len(s.buf) - deflate.WindowSize)
}

func (s *sink) flushN(n int) error {
	if n <= 0 {
		return nil
	}
	chunk := s.buf[:n]
	s.crc = crc32.Update(s.crc, chunk)
	if _, err := s.dst.Write(chunk); err != nil {
		return err
	}
	rest := len(s.buf) - n
	copy(s.buf, s.buf[n:])
	s.buf = s.buf[:rest]
	return nil
}

func (s *sink) finish() error {
	return s.flushN(len(s.buf))
}

// Decompress inflates every gzip member in r into dst, checking each trailer,
// and returns the first member's header.
func Decompress(dst io.Writer, r io.Reader) (Header, error) {
	br := bufio.NewReader(r)
	var first Header
	members := 0
	for {
		if _, err := br.Peek(1); err != nil {
			if err == io.EOF {
				if members == 0 {
					return first, ErrHeader
				}
				return first, nil
			}
			return first, err
		}
		hdr, err := decompressMember(dst, br)
		if err != nil {
			return first, err
		}
		if members == 0 {
			first = hdr
		}
		members++
	}
}

func decompressMember(dst io.Writer, br *bufio.Reader) (Header, error) {
	var hdr Header
	head := make([]byte, 10)
	if _, err := io.ReadFull(br, head); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return hdr, ErrHeader
		}
		return hdr, err
	}
	if head[0] != 0x1f || head[1] != 0x8b {
		return hdr, fmt.Errorf("%w: bad magic %#x %#x", ErrHeader, head[0], head[1])
	}
	if head[2] != cmDeflate {
		return hdr, fmt.Errorf("%w: unsupported compression method %d", ErrHeader, head[2])
	}
	flg := head[3]
	hdr.ModTime = uint32(head[4]) | uint32(head[5])<<8 | uint32(head[6])<<16 | uint32(head[7])<<24

	if flg&flagExtra != 0 {
		var x [2]byte
		if _, err := io.ReadFull(br, x[:]); err != nil {
			return hdr, unexpected(err)
		}
		xlen := int(x[0]) | int(x[1])<<8
		if _, err := br.Discard(xlen); err != nil {
			return hdr, unexpected(err)
		}
	}
	if flg&flagName != 0 {
		name, err := readCString(br)
		if err != nil {
			return hdr, err
		}
		hdr.Name = name
	}
	if flg&flagComment != 0 {
		if _, err := readCString(br); err != nil {
			return hdr, err
		}
	}
	if flg&flagHCRC != 0 {
		if _, err := br.Discard(2); err != nil {
			return hdr, unexpected(err)
		}
	}

	s := newSink(dst)
	bitr := bitio.NewReader(br)
	if err := deflate.Inflate(bitr, s); err != nil {
		return hdr, err
	}
	if err := s.finish(); err != nil {
		return hdr, err
	}

	// The trailer has to come through the bit reader, which may already have
	// buffered some of its bytes.
	bitr.Align()
	var tail [8]byte
	for i := 0; i < 8; i++ {
		b, err := bitr.ReadByteAligned()
		if err != nil {
			return hdr, unexpected(err)
		}
		tail[i] = b
	}
	wantCRC := uint32(tail[0]) | uint32(tail[1])<<8 | uint32(tail[2])<<16 | uint32(tail[3])<<24
	wantSize := uint32(tail[4]) | uint32(tail[5])<<8 | uint32(tail[6])<<16 | uint32(tail[7])<<24

	if s.crc != wantCRC {
		return hdr, ErrChecksum
	}
	if uint32(s.total) != wantSize {
		return hdr, ErrSize
	}
	return hdr, nil
}

func readCString(br *bufio.Reader) (string, error) {
	var out []byte
	for {
		b, err := br.ReadByte()
		if err != nil {
			return "", unexpected(err)
		}
		if b == 0 {
			return string(out), nil
		}
		out = append(out, b)
	}
}

func unexpected(err error) error {
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}
