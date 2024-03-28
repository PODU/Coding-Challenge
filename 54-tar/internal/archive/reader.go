package archive

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"mytar/internal/ustar"
)

// Reader iterates the members of a USTAR stream.
type Reader struct {
	r         io.Reader
	remaining int64 // unread body bytes of the current member
	pad       int   // padding bytes after the body to skip before next header
	err       error
}

// NewReader returns a Reader reading from r.
func NewReader(r io.Reader) *Reader { return &Reader{r: r} }

// Next advances to the next member and returns its header, or io.EOF at the
// end of the archive. GNU 'L' long-name records are applied to the header that
// follows them rather than returned.
func (r *Reader) Next() (*ustar.Header, error) {
	if r.err != nil {
		return nil, r.err
	}
	if err := r.skipBody(); err != nil {
		r.err = err
		return nil, err
	}

	var longName string
	for {
		h, err := r.readHeader()
		if err != nil {
			r.err = err
			return nil, err
		}
		r.remaining = h.Size
		r.pad = int((ustar.BlockSize - h.Size%ustar.BlockSize) % ustar.BlockSize)

		if h.Typeflag == ustar.TypeGNULong {
			data, err := io.ReadAll(r)
			if err != nil {
				r.err = err
				return nil, err
			}
			if err := r.skipBody(); err != nil {
				r.err = err
				return nil, err
			}
			longName = strings.TrimRight(string(data), "\x00")
			continue
		}

		if longName != "" {
			h.Name = longName
		}
		return h, nil
	}
}

// readHeader reads one 512-byte header block, treating the first zero block as
// end-of-archive (io.EOF).
func (r *Reader) readHeader() (*ustar.Header, error) {
	block := make([]byte, ustar.BlockSize)
	if _, err := io.ReadFull(r.r, block); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, io.EOF
		}
		return nil, err
	}
	h, err := ustar.ParseHeader(block)
	if errors.Is(err, ustar.ErrZeroBlock) {
		// Like GNU and BSD tar, stop at the first zero block without checking
		// for the second.
		return nil, io.EOF
	}
	if err != nil {
		return nil, err
	}
	return h, nil
}

// Read returns body bytes of the current member.
func (r *Reader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.r.Read(p)
	r.remaining -= int64(n)
	if err == io.EOF && r.remaining > 0 {
		return n, io.ErrUnexpectedEOF
	}
	return n, err
}

// skipBody discards the unread body and trailing padding of the current member.
func (r *Reader) skipBody() error {
	if r.remaining > 0 {
		if _, err := io.CopyN(io.Discard, r.r, r.remaining); err != nil {
			return fmt.Errorf("archive: truncated member body: %w", err)
		}
		r.remaining = 0
	}
	if r.pad > 0 {
		if _, err := io.CopyN(io.Discard, r.r, int64(r.pad)); err != nil {
			return fmt.Errorf("archive: truncated block padding: %w", err)
		}
		r.pad = 0
	}
	return nil
}
