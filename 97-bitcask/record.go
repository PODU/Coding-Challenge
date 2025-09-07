package bitcask

import (
	"encoding/binary"
	"hash/crc32"
)

// Data record layout (little-endian):
//
//	offset  size  field
//	  0      4    crc32      CRC-32 (Castagnoli) over bytes [4 .. end of record]
//	  4      8    timestamp  Unix nanoseconds of the write
//	 12      1    flags      bit0 = tombstone (deleted); other bits reserved 0
//	 13      4    key_size   uint32, length of key in bytes
//	 17      4    value_size uint32, length of value in bytes (0 for tombstone)
//	 21    ksz    key        raw key bytes
//	21+ksz vsz    value      raw value bytes (absent for tombstone)
const (
	headerSize  = 21
	crcOffset   = 0
	crcSize     = 4
	tsOffset    = 4
	flagsOffset = 12
	ksizeOffset = 13
	vsizeOffset = 17

	flagTombstone byte = 1 << 0
)

// crcTable is the Castagnoli table (hardware accelerated on most CPUs).
var crcTable = crc32.MakeTable(crc32.Castagnoli)

// header is the fixed-size prefix of a data record.
type header struct {
	crc       uint32
	timestamp int64
	flags     byte
	keySize   uint32
	valueSize uint32
}

func (h header) isTombstone() bool { return h.flags&flagTombstone != 0 }

func (h header) recordLen() int64 {
	return headerSize + int64(h.keySize) + int64(h.valueSize)
}

// encodeRecord serializes a record, with the CRC covering everything after
// the CRC field.
func encodeRecord(timestamp int64, flags byte, key, value []byte) []byte {
	total := headerSize + len(key) + len(value)
	buf := make([]byte, total)
	binary.LittleEndian.PutUint64(buf[tsOffset:], uint64(timestamp))
	buf[flagsOffset] = flags
	binary.LittleEndian.PutUint32(buf[ksizeOffset:], uint32(len(key)))
	binary.LittleEndian.PutUint32(buf[vsizeOffset:], uint32(len(value)))
	copy(buf[headerSize:], key)
	copy(buf[headerSize+len(key):], value)
	crc := crc32.Checksum(buf[crcSize:], crcTable)
	binary.LittleEndian.PutUint32(buf[crcOffset:], crc)
	return buf
}

// decodeHeader parses a 21-byte header. buf must be at least headerSize long.
func decodeHeader(buf []byte) header {
	return header{
		crc:       binary.LittleEndian.Uint32(buf[crcOffset:]),
		timestamp: int64(binary.LittleEndian.Uint64(buf[tsOffset:])),
		flags:     buf[flagsOffset],
		keySize:   binary.LittleEndian.Uint32(buf[ksizeOffset:]),
		valueSize: binary.LittleEndian.Uint32(buf[vsizeOffset:]),
	}
}

// verifyCRC reports whether a full record's checksum matches. A mismatch
// means the record is corrupt or was only partly written.
func verifyCRC(buf []byte) bool {
	if len(buf) < headerSize {
		return false
	}
	stored := binary.LittleEndian.Uint32(buf[crcOffset:])
	return stored == crc32.Checksum(buf[crcSize:], crcTable)
}
