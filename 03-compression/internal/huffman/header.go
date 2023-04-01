package huffman

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Magic identifies a HUF1 file; it also serves as the format version.
var Magic = [4]byte{'H', 'U', 'F', '1'}

// ErrBadMagic means the input doesn't start with Magic.
var ErrBadMagic = errors.New("huffman: bad magic (not a HUF1 file)")

// ErrCorrupt means the container is truncated or malformed.
var ErrCorrupt = errors.New("huffman: corrupt or truncated input")

// The decoder rebuilds the same tree from freq, so the tree itself is never
// stored.
type header struct {
	symbolCount uint64
	freq        FreqTable
}

// writeHeader writes:
//
//	MAGIC        4 bytes  "HUF1"
//	SYMBOL_COUNT 8 bytes  uint64 BE, original byte count
//	TABLE_LEN    2 bytes  uint16 BE, number of distinct symbols (0..256)
//	ENTRIES      TABLE_LEN * { symbol:1, freq:uint64 BE }
func writeHeader(w io.Writer, symbolCount uint64, freq *FreqTable) error {
	if _, err := w.Write(Magic[:]); err != nil {
		return err
	}
	var u64 [8]byte
	binary.BigEndian.PutUint64(u64[:], symbolCount)
	if _, err := w.Write(u64[:]); err != nil {
		return err
	}

	tableLen := freq.DistinctCount()
	var u16 [2]byte
	binary.BigEndian.PutUint16(u16[:], uint16(tableLen))
	if _, err := w.Write(u16[:]); err != nil {
		return err
	}

	for sym := 0; sym < 256; sym++ {
		if freq[sym] == 0 {
			continue
		}
		var entry [9]byte
		entry[0] = byte(sym)
		binary.BigEndian.PutUint64(entry[1:], freq[sym])
		if _, err := w.Write(entry[:]); err != nil {
			return err
		}
	}
	return nil
}

// readHeader parses and sanity-checks the header written by writeHeader.
func readHeader(r io.Reader) (header, error) {
	var h header

	var magic [4]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return h, ErrBadMagic
		}
		return h, err
	}
	if magic != Magic {
		return h, ErrBadMagic
	}

	var u64 [8]byte
	if _, err := io.ReadFull(r, u64[:]); err != nil {
		return h, ErrCorrupt
	}
	h.symbolCount = binary.BigEndian.Uint64(u64[:])

	var u16 [2]byte
	if _, err := io.ReadFull(r, u16[:]); err != nil {
		return h, ErrCorrupt
	}
	tableLen := int(binary.BigEndian.Uint16(u16[:]))
	if tableLen > 256 {
		return h, fmt.Errorf("%w: table length %d exceeds 256", ErrCorrupt, tableLen)
	}

	var total uint64
	seen := make(map[byte]bool, tableLen)
	for i := 0; i < tableLen; i++ {
		var entry [9]byte
		if _, err := io.ReadFull(r, entry[:]); err != nil {
			return h, ErrCorrupt
		}
		sym := entry[0]
		if seen[sym] {
			return h, fmt.Errorf("%w: duplicate symbol %d in table", ErrCorrupt, sym)
		}
		seen[sym] = true
		freq := binary.BigEndian.Uint64(entry[1:])
		if freq == 0 {
			return h, fmt.Errorf("%w: zero frequency for symbol %d", ErrCorrupt, sym)
		}
		h.freq[sym] = freq
		total += freq
	}

	if total != h.symbolCount {
		return h, fmt.Errorf("%w: frequency sum %d != symbol count %d", ErrCorrupt, total, h.symbolCount)
	}
	if tableLen == 0 && h.symbolCount != 0 {
		return h, fmt.Errorf("%w: empty table with non-zero symbol count", ErrCorrupt)
	}
	return h, nil
}
