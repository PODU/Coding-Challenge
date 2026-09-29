package dbm

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// On-disk layout (all integers little-endian).
//
// The file is an array of fixed-size pages. Page 0 is the header. Every
// other page begins with a 24-byte page header whose first byte says what
// the page is:
//
//	pageFree      a page on the free list; next -> next free page
//	pageDir       one slice of the directory: an array of uint64 page numbers
//	pageBucket    the head page of a bucket
//	pageOverflow  a continuation page of a bucket
//
// A bucket is a byte stream of entries spread over its head page and any
// overflow pages chained through next. Each entry is
//
//	uvarint keyLen | uvarint valLen | uvarint expires | key | value
//
// so keys and values are arbitrary bytes of arbitrary length: an entry
// larger than a page simply continues into the next overflow page. The
// head page records the bucket's local depth and entry count; every page
// records how many payload bytes it holds.

const (
	magic       = "CCDBM\x00"
	version     = 1
	headerSize  = 64
	pageHdrSize = 24

	// DefaultPageSize is used by Create when Options.PageSize is zero.
	DefaultPageSize = 4096
	// MinPageSize and MaxPageSize bound Options.PageSize.
	MinPageSize = 128
	MaxPageSize = 1 << 20

	// maxGlobalDepth caps the directory at 2^24 slots (128 MiB of
	// pointers); beyond that a full bucket grows an overflow chain instead.
	maxGlobalDepth = 24
	// splitWindow: a bucket is only split if its entries plus the incoming
	// key differ somewhere in the next splitWindow hash bits, otherwise
	// the split would move nothing and just double the directory.
	splitWindow = 4
	// maxSplitsPerInsert bounds the work a single Put can do.
	maxSplitsPerInsert = 8
)

const (
	pageFree     = 1
	pageDir      = 2
	pageBucket   = 3
	pageOverflow = 4
)

// ErrCorrupt is wrapped by every error caused by an inconsistent file.
var ErrCorrupt = errors.New("dbm: database file is corrupt")

func corrupt(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrCorrupt, fmt.Sprintf(format, args...))
}

// header is the contents of page 0.
type header struct {
	pageSize    uint32
	hashID      uint32
	globalDepth uint32
	dirPages    uint32
	dirPage     uint64
	numPages    uint64
	numKeys     uint64
	freeHead    uint64
}

func (h header) encode(buf []byte) {
	copy(buf, magic)
	binary.LittleEndian.PutUint16(buf[6:], version)
	binary.LittleEndian.PutUint32(buf[8:], h.pageSize)
	binary.LittleEndian.PutUint32(buf[12:], h.hashID)
	binary.LittleEndian.PutUint32(buf[16:], h.globalDepth)
	binary.LittleEndian.PutUint32(buf[20:], h.dirPages)
	binary.LittleEndian.PutUint64(buf[24:], h.dirPage)
	binary.LittleEndian.PutUint64(buf[32:], h.numPages)
	binary.LittleEndian.PutUint64(buf[40:], h.numKeys)
	binary.LittleEndian.PutUint64(buf[48:], h.freeHead)
}

func decodeHeader(buf []byte) (header, error) {
	var h header
	if len(buf) < headerSize || string(buf[:len(magic)]) != magic {
		return h, corrupt("bad magic (not a ccdbm file)")
	}
	if v := binary.LittleEndian.Uint16(buf[6:]); v != version {
		return h, fmt.Errorf("dbm: unsupported file version %d (want %d)", v, version)
	}
	h.pageSize = binary.LittleEndian.Uint32(buf[8:])
	h.hashID = binary.LittleEndian.Uint32(buf[12:])
	h.globalDepth = binary.LittleEndian.Uint32(buf[16:])
	h.dirPages = binary.LittleEndian.Uint32(buf[20:])
	h.dirPage = binary.LittleEndian.Uint64(buf[24:])
	h.numPages = binary.LittleEndian.Uint64(buf[32:])
	h.numKeys = binary.LittleEndian.Uint64(buf[40:])
	h.freeHead = binary.LittleEndian.Uint64(buf[48:])
	if h.pageSize < MinPageSize || h.pageSize > MaxPageSize || h.pageSize&(h.pageSize-1) != 0 {
		return h, corrupt("bad page size %d", h.pageSize)
	}
	if h.globalDepth > maxGlobalDepth {
		return h, corrupt("global depth %d out of range", h.globalDepth)
	}
	return h, nil
}

// pageHdr is the 24-byte prefix of every page after the header page.
type pageHdr struct {
	typ   uint8
	depth uint8
	count uint32 // entries in the bucket (head pages only)
	used  uint32 // payload bytes in this page
	next  uint64 // next page in chain, 0 = none
}

func (p pageHdr) encode(buf []byte) {
	buf[0] = p.typ
	buf[1] = p.depth
	buf[2], buf[3] = 0, 0
	binary.LittleEndian.PutUint32(buf[4:], p.count)
	binary.LittleEndian.PutUint32(buf[8:], p.used)
	binary.LittleEndian.PutUint32(buf[12:], 0)
	binary.LittleEndian.PutUint64(buf[16:], p.next)
}

func decodePageHdr(buf []byte) pageHdr {
	return pageHdr{
		typ:   buf[0],
		depth: buf[1],
		count: binary.LittleEndian.Uint32(buf[4:]),
		used:  binary.LittleEndian.Uint32(buf[8:]),
		next:  binary.LittleEndian.Uint64(buf[16:]),
	}
}

// entry is one key/value pair. expires is a Unix timestamp in seconds, or
// 0 for no expiry.
type entry struct {
	key, val []byte
	expires  uint64
}

func (e entry) size() int {
	return uvarintLen(uint64(len(e.key))) + uvarintLen(uint64(len(e.val))) + uvarintLen(e.expires) + len(e.key) + len(e.val)
}

func uvarintLen(v uint64) int {
	n := 1
	for v >= 0x80 {
		v >>= 7
		n++
	}
	return n
}

func entriesSize(es []entry) int {
	n := 0
	for _, e := range es {
		n += e.size()
	}
	return n
}

func encodeEntries(es []entry) []byte {
	out := make([]byte, 0, entriesSize(es))
	for _, e := range es {
		out = binary.AppendUvarint(out, uint64(len(e.key)))
		out = binary.AppendUvarint(out, uint64(len(e.val)))
		out = binary.AppendUvarint(out, e.expires)
		out = append(out, e.key...)
		out = append(out, e.val...)
	}
	return out
}

func decodeEntries(stream []byte, count uint32) ([]entry, error) {
	es := make([]entry, 0, count)
	for i := uint32(0); i < count; i++ {
		klen, n := binary.Uvarint(stream)
		if n <= 0 {
			return nil, corrupt("bad key length in bucket")
		}
		stream = stream[n:]
		vlen, n := binary.Uvarint(stream)
		if n <= 0 {
			return nil, corrupt("bad value length in bucket")
		}
		stream = stream[n:]
		exp, n := binary.Uvarint(stream)
		if n <= 0 {
			return nil, corrupt("bad expiry in bucket")
		}
		stream = stream[n:]
		if klen > uint64(len(stream)) || vlen > uint64(len(stream))-klen {
			return nil, corrupt("entry overruns bucket data")
		}
		e := entry{expires: exp}
		e.key = append([]byte{}, stream[:klen]...)
		e.val = append([]byte{}, stream[klen:klen+vlen]...)
		stream = stream[klen+vlen:]
		es = append(es, e)
	}
	if len(stream) != 0 {
		return nil, corrupt("%d trailing bytes in bucket", len(stream))
	}
	return es, nil
}
