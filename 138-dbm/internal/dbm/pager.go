package dbm

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// pager reads and writes fixed-size pages and manages the free list. It
// knows nothing about buckets or the directory.
type pager struct {
	f        *os.File
	pageSize int
	numPages uint64 // pages allocated (file length / pageSize)
	freeHead uint64 // first free page, 0 = none
	readOnly bool
}

func (p *pager) payload() int { return p.pageSize - pageHdrSize }

func (p *pager) read(n uint64) ([]byte, error) {
	if n == 0 || n >= p.numPages {
		return nil, corrupt("page %d out of range (file has %d pages)", n, p.numPages)
	}
	buf := make([]byte, p.pageSize)
	if _, err := p.f.ReadAt(buf, int64(n)*int64(p.pageSize)); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, corrupt("page %d is beyond end of file", n)
		}
		return nil, fmt.Errorf("dbm: read page %d: %w", n, err)
	}
	return buf, nil
}

// readHdr reads only the page header of page n.
func (p *pager) readHdr(n uint64) (pageHdr, error) {
	if n == 0 || n >= p.numPages {
		return pageHdr{}, corrupt("page %d out of range (file has %d pages)", n, p.numPages)
	}
	var buf [pageHdrSize]byte
	if _, err := p.f.ReadAt(buf[:], int64(n)*int64(p.pageSize)); err != nil {
		if errors.Is(err, io.EOF) {
			return pageHdr{}, corrupt("page %d is beyond end of file", n)
		}
		return pageHdr{}, fmt.Errorf("dbm: read page %d: %w", n, err)
	}
	return decodePageHdr(buf[:]), nil
}

func (p *pager) write(n uint64, buf []byte) error {
	if p.readOnly {
		return ErrReadOnly
	}
	if len(buf) != p.pageSize {
		panic("dbm: write of wrong-sized page")
	}
	if _, err := p.f.WriteAt(buf, int64(n)*int64(p.pageSize)); err != nil {
		return fmt.Errorf("dbm: write page %d: %w", n, err)
	}
	return nil
}

// alloc returns a page number, reusing a free page when one exists and
// growing the file otherwise. The page's contents are undefined until the
// caller writes it.
func (p *pager) alloc() (uint64, error) {
	if p.freeHead != 0 {
		n := p.freeHead
		hdr, err := p.readHdr(n)
		if err != nil {
			return 0, err
		}
		if hdr.typ != pageFree {
			return 0, corrupt("free list points at page %d of type %d", n, hdr.typ)
		}
		p.freeHead = hdr.next
		return n, nil
	}
	n := p.numPages
	p.numPages++
	return n, nil
}

// allocRun reserves count consecutive pages at the end of the file (the
// directory must be contiguous).
func (p *pager) allocRun(count uint64) uint64 {
	n := p.numPages
	p.numPages += count
	return n
}

// free pushes page n onto the free list.
func (p *pager) free(n uint64) error {
	buf := make([]byte, p.pageSize)
	pageHdr{typ: pageFree, next: p.freeHead}.encode(buf)
	if err := p.write(n, buf); err != nil {
		return err
	}
	p.freeHead = n
	return nil
}

func (p *pager) sync() error {
	if p.readOnly {
		return nil
	}
	return p.f.Sync()
}
