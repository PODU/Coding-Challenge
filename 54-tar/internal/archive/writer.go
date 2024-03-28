// Package archive layers 512-byte block framing on top of the ustar codec. It
// knows about block padding and the two trailing zero blocks, but nothing about
// the filesystem.
package archive

import (
	"fmt"
	"io"
	"time"

	"mytar/internal/ustar"
)

var zeroBlock = make([]byte, ustar.BlockSize)

// Writer streams USTAR members to an underlying io.Writer.
type Writer struct {
	w         io.Writer
	remaining int64 // body bytes still expected for the current member
	pad       int   // zero bytes owed to finish the current member's last block
	closed    bool
}

// NewWriter returns a Writer that writes to w.
func NewWriter(w io.Writer) *Writer { return &Writer{w: w} }

// WriteHeader writes h's header block and prepares the Writer to accept h.Size
// body bytes via Write. Any padding owed by the previous member is flushed
// first.
func (w *Writer) WriteHeader(h *ustar.Header) error {
	if w.closed {
		return fmt.Errorf("archive: WriteHeader after Close")
	}
	if err := w.finishMember(); err != nil {
		return err
	}
	block, err := h.Marshal()
	if err != nil {
		return err
	}
	if _, err := w.w.Write(block[:]); err != nil {
		return err
	}
	w.remaining = h.Size
	w.pad = int((ustar.BlockSize - h.Size%ustar.BlockSize) % ustar.BlockSize)
	return nil
}

// WriteGNULongName emits a GNU 'L' extension record whose body holds fullName,
// used when the name cannot fit in the USTAR name/prefix fields.
func (w *Writer) WriteGNULongName(fullName string) error {
	body := append([]byte(fullName), 0)
	h := &ustar.Header{
		Name:     ustar.GNULongName,
		Size:     int64(len(body)),
		ModTime:  time.Unix(0, 0),
		Typeflag: ustar.TypeGNULong,
	}
	if err := w.WriteHeader(h); err != nil {
		return err
	}
	_, err := w.Write(body)
	return err
}

// Write appends body bytes for the current member, enforcing the declared Size.
func (w *Writer) Write(p []byte) (int, error) {
	if w.closed {
		return 0, fmt.Errorf("archive: Write after Close")
	}
	if int64(len(p)) > w.remaining {
		return 0, fmt.Errorf("archive: write of %d bytes exceeds remaining %d for member body", len(p), w.remaining)
	}
	n, err := w.w.Write(p)
	w.remaining -= int64(n)
	return n, err
}

// finishMember verifies the current member body is complete and writes its
// block padding.
func (w *Writer) finishMember() error {
	if w.remaining != 0 {
		return fmt.Errorf("archive: %d body bytes still expected before next header", w.remaining)
	}
	if w.pad > 0 {
		if _, err := w.w.Write(zeroBlock[:w.pad]); err != nil {
			return err
		}
		w.pad = 0
	}
	return nil
}

// Close pads the final member and writes the two trailing zero blocks.
func (w *Writer) Close() error {
	if w.closed {
		return nil
	}
	if err := w.finishMember(); err != nil {
		return err
	}
	w.closed = true
	if _, err := w.w.Write(zeroBlock); err != nil {
		return err
	}
	_, err := w.w.Write(zeroBlock)
	return err
}
