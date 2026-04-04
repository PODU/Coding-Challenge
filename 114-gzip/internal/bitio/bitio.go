// Package bitio reads and writes DEFLATE's bit stream, which packs bits into
// each byte starting from the least significant one. Huffman codes go out
// most-significant bit first, so the encoder reverses them before WriteBits.
package bitio

import (
	"bufio"
	"io"
)

type Writer struct {
	w    io.Writer
	acc  uint64
	n    uint // bits pending in acc
	buf  []byte
	bits uint64 // total written so far, used for block cost estimates
	err  error
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w, buf: make([]byte, 0, 4096)}
}

// WriteBits writes the low n bits of v, least-significant bit first.
func (w *Writer) WriteBits(v uint32, n uint) {
	w.acc |= uint64(v&((1<<n)-1)) << w.n
	w.n += n
	w.bits += uint64(n)
	for w.n >= 8 {
		w.buf = append(w.buf, byte(w.acc))
		w.acc >>= 8
		w.n -= 8
		if len(w.buf) >= 4096 {
			w.flushBuf()
		}
	}
}

// BitCount returns the total number of bits written so far.
func (w *Writer) BitCount() uint64 { return w.bits }

// Align pads with zero bits to the next byte boundary.
func (w *Writer) Align() {
	if r := w.n % 8; r != 0 {
		w.WriteBits(0, 8-r)
	}
}

// WriteBytes writes raw bytes. The writer must be byte-aligned.
func (w *Writer) WriteBytes(p []byte) {
	w.buf = append(w.buf, p...)
	w.bits += uint64(len(p)) * 8
	if len(w.buf) >= 4096 {
		w.flushBuf()
	}
}

func (w *Writer) flushBuf() {
	if w.err != nil || len(w.buf) == 0 {
		return
	}
	if _, err := w.w.Write(w.buf); err != nil {
		w.err = err
	}
	w.buf = w.buf[:0]
}

// Flush writes out buffered bytes. Call Align first or the last partial byte
// is lost.
func (w *Writer) Flush() error {
	w.flushBuf()
	return w.err
}

type Reader struct {
	r   io.ByteReader
	acc uint64
	n   uint
}

func NewReader(r io.Reader) *Reader {
	br, ok := r.(io.ByteReader)
	if !ok {
		br = bufio.NewReader(r)
	}
	return &Reader{r: br}
}

// ReadBits reads n bits (n <= 32), least-significant bit first.
func (r *Reader) ReadBits(n uint) (uint32, error) {
	for r.n < n {
		b, err := r.r.ReadByte()
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return 0, err
		}
		r.acc |= uint64(b) << r.n
		r.n += 8
	}
	v := uint32(r.acc & ((1 << n) - 1))
	r.acc >>= n
	r.n -= n
	return v, nil
}

func (r *Reader) ReadBit() (uint32, error) {
	if r.n == 0 {
		b, err := r.r.ReadByte()
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return 0, err
		}
		r.acc = uint64(b)
		r.n = 8
	}
	v := uint32(r.acc & 1)
	r.acc >>= 1
	r.n--
	return v, nil
}

// Align discards bits up to the next byte boundary.
func (r *Reader) Align() {
	drop := r.n % 8
	r.acc >>= drop
	r.n -= drop
}

// ReadByteAligned returns the next whole byte. Call Align first.
func (r *Reader) ReadByteAligned() (byte, error) {
	if r.n >= 8 {
		b := byte(r.acc)
		r.acc >>= 8
		r.n -= 8
		return b, nil
	}
	return r.r.ReadByte()
}
