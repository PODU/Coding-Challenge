package bitcask

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
)

// Hint record layout (little-endian):
//
//	size  field
//	  8   timestamp
//	  4   key_size
//	  4   value_size
//	  4   record_size
//	  8   value_pos   (record start offset in the paired .cask file)
//	 ksz  key
const hintHeaderSize = 28

// errHintFallback signals that a hint file could not be used and the paired
// data file must be fully scanned instead. Hints are an optimization, never the
// source of truth.
var errHintFallback = errors.New("bitcask: hint unusable, falling back to scan")

func encodeHint(ts int64, ksize, vsize, recSize uint32, valuePos int64, key []byte) []byte {
	buf := make([]byte, hintHeaderSize+len(key))
	binary.LittleEndian.PutUint64(buf[0:], uint64(ts))
	binary.LittleEndian.PutUint32(buf[8:], ksize)
	binary.LittleEndian.PutUint32(buf[12:], vsize)
	binary.LittleEndian.PutUint32(buf[16:], recSize)
	binary.LittleEndian.PutUint64(buf[20:], uint64(valuePos))
	copy(buf[hintHeaderSize:], key)
	return buf
}

// loadHint fills the keydir from a hint file. It returns errHintFallback if the
// hint is missing, corrupt, or doesn't cover the whole data file (because the
// file was appended to after the hint was written); the caller then scans the
// data file instead.
func (b *Bitcask) loadHint(id uint32, path string, dataSize int64, latest map[string]int64) error {
	f, err := os.Open(path)
	if err != nil {
		return errHintFallback
	}
	defer f.Close()

	r := io.Reader(f)
	hdr := make([]byte, hintHeaderSize)
	var covered int64
	for {
		_, err := io.ReadFull(r, hdr)
		if err == io.EOF {
			break
		}
		if err != nil {
			return errHintFallback // torn/short hint
		}
		ts := int64(binary.LittleEndian.Uint64(hdr[0:]))
		ksize := binary.LittleEndian.Uint32(hdr[8:])
		vsize := binary.LittleEndian.Uint32(hdr[12:])
		recSize := binary.LittleEndian.Uint32(hdr[16:])
		valuePos := int64(binary.LittleEndian.Uint64(hdr[20:]))

		key := make([]byte, ksize)
		if _, err := io.ReadFull(r, key); err != nil {
			return errHintFallback
		}
		// Cross-check against the paired data file.
		if valuePos < 0 || recSize == 0 || valuePos+int64(recSize) > dataSize {
			return errHintFallback
		}
		b.applyPut(latest, string(key), ts, Entry{
			FileID:    id,
			ValuePos:  valuePos,
			ValueSize: vsize,
			RecSize:   recSize,
			Timestamp: ts,
		})
		if end := valuePos + int64(recSize); end > covered {
			covered = end
		}
	}
	// The hint must describe the whole data file; otherwise newer records exist
	// on disk that the hint does not know about.
	if covered != dataSize {
		return errHintFallback
	}
	return nil
}
