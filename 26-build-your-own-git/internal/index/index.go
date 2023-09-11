// Package index implements Git's staging area: the binary DIRC v2 index format,
// including exact field layout, entry ordering, 8-byte NUL padding, and the
// trailing SHA-1 checksum over the whole file.
package index

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const signature = "DIRC"
const version = 2

// Mode bits for index entries (Git stores the full st_mode style value).
const (
	ModeRegular uint32 = 0o100644
	ModeExec    uint32 = 0o100755
	ModeSymlink uint32 = 0o120000
)

// Entry is a single staged file. Fields with no Windows equivalent
// (Dev/Ino/Uid/Gid) are stored as 0; Git tolerates and refreshes them.
type Entry struct {
	CtimeSec  uint32
	CtimeNsec uint32
	MtimeSec  uint32
	MtimeNsec uint32
	Dev       uint32
	Ino       uint32
	Mode      uint32
	Uid       uint32
	Gid       uint32
	Size      uint32
	OID       [20]byte
	Flags     uint16
	Path      string // POSIX-style, forward slashes
}

// OIDHex returns the entry OID as a hex string.
func (e Entry) OIDHex() string { return hex.EncodeToString(e.OID[:]) }

// Index is an in-memory representation of the staging area.
type Index struct {
	Entries []Entry
}

func New() *Index { return &Index{} }

// SetOIDHex sets an entry's OID from a hex string.
func (e *Entry) SetOIDHex(h string) error {
	raw, err := hex.DecodeString(h)
	if err != nil || len(raw) != 20 {
		return fmt.Errorf("bad oid %q", h)
	}
	copy(e.OID[:], raw)
	return nil
}

// makeFlags builds the 16-bit flags field: stage 0, name length capped at 0xFFF.
func makeFlags(path string) uint16 {
	n := len(path)
	if n > 0xFFF {
		n = 0xFFF
	}
	return uint16(n)
}

// Add inserts or replaces an entry, keeping entries sorted by path.
func (idx *Index) Add(e Entry) {
	e.Flags = makeFlags(e.Path)
	for i := range idx.Entries {
		if idx.Entries[i].Path == e.Path {
			idx.Entries[i] = e
			return
		}
	}
	idx.Entries = append(idx.Entries, e)
	idx.sort()
}

// Remove deletes an entry by path; reports whether it existed.
func (idx *Index) Remove(path string) bool {
	for i := range idx.Entries {
		if idx.Entries[i].Path == path {
			idx.Entries = append(idx.Entries[:i], idx.Entries[i+1:]...)
			return true
		}
	}
	return false
}

// Get returns the entry for a path.
func (idx *Index) Get(path string) (Entry, bool) {
	for _, e := range idx.Entries {
		if e.Path == path {
			return e, true
		}
	}
	return Entry{}, false
}

func (idx *Index) sort() {
	sort.SliceStable(idx.Entries, func(i, j int) bool {
		return idx.Entries[i].Path < idx.Entries[j].Path
	})
}

// Read loads an index file. A missing file yields an empty index.
func Read(path string) (*Index, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return New(), nil
		}
		return nil, err
	}
	return Parse(data)
}

// Parse decodes index bytes.
func Parse(data []byte) (*Index, error) {
	if len(data) < 12+20 {
		return nil, fmt.Errorf("index too short")
	}
	if string(data[:4]) != signature {
		return nil, fmt.Errorf("bad index signature %q", data[:4])
	}
	ver := binary.BigEndian.Uint32(data[4:8])
	if ver != version {
		return nil, fmt.Errorf("unsupported index version %d", ver)
	}
	count := binary.BigEndian.Uint32(data[8:12])

	body := data[:len(data)-20]
	want := data[len(data)-20:]
	got := sha1.Sum(body)
	if !bytes.Equal(got[:], want) {
		return nil, fmt.Errorf("index checksum mismatch")
	}

	idx := New()
	off := 12
	for i := uint32(0); i < count; i++ {
		if off+62 > len(body) {
			return nil, fmt.Errorf("index entry %d: truncated", i)
		}
		var e Entry
		e.CtimeSec = binary.BigEndian.Uint32(body[off+0:])
		e.CtimeNsec = binary.BigEndian.Uint32(body[off+4:])
		e.MtimeSec = binary.BigEndian.Uint32(body[off+8:])
		e.MtimeNsec = binary.BigEndian.Uint32(body[off+12:])
		e.Dev = binary.BigEndian.Uint32(body[off+16:])
		e.Ino = binary.BigEndian.Uint32(body[off+20:])
		e.Mode = binary.BigEndian.Uint32(body[off+24:])
		e.Uid = binary.BigEndian.Uint32(body[off+28:])
		e.Gid = binary.BigEndian.Uint32(body[off+32:])
		e.Size = binary.BigEndian.Uint32(body[off+36:])
		copy(e.OID[:], body[off+40:off+60])
		e.Flags = binary.BigEndian.Uint16(body[off+60:])
		nameLen := int(e.Flags & 0xFFF)

		start := off + 62
		var path string
		if nameLen < 0xFFF {
			if start+nameLen > len(body) {
				return nil, fmt.Errorf("index entry %d: name overruns", i)
			}
			path = string(body[start : start+nameLen])
		} else {
			// Name length >= 0xFFF: read until NUL.
			nul := bytes.IndexByte(body[start:], 0)
			if nul < 0 {
				return nil, fmt.Errorf("index entry %d: unterminated name", i)
			}
			path = string(body[start : start+nul])
			nameLen = len(path)
		}
		e.Path = path

		entryLen := 62 + nameLen
		padded := (entryLen + 8) &^ 7 // pad to a multiple of 8 with at least one NUL
		off += padded
		idx.Entries = append(idx.Entries, e)
	}
	idx.sort()
	return idx, nil
}

// Bytes serializes the index to its on-disk byte form (with checksum).
func (idx *Index) Bytes() []byte {
	idx.sort()
	var buf bytes.Buffer
	buf.WriteString(signature)
	tmp4 := make([]byte, 4)
	binary.BigEndian.PutUint32(tmp4, version)
	buf.Write(tmp4)
	binary.BigEndian.PutUint32(tmp4, uint32(len(idx.Entries)))
	buf.Write(tmp4)

	for _, e := range idx.Entries {
		var fixed [62]byte
		binary.BigEndian.PutUint32(fixed[0:], e.CtimeSec)
		binary.BigEndian.PutUint32(fixed[4:], e.CtimeNsec)
		binary.BigEndian.PutUint32(fixed[8:], e.MtimeSec)
		binary.BigEndian.PutUint32(fixed[12:], e.MtimeNsec)
		binary.BigEndian.PutUint32(fixed[16:], e.Dev)
		binary.BigEndian.PutUint32(fixed[20:], e.Ino)
		binary.BigEndian.PutUint32(fixed[24:], e.Mode)
		binary.BigEndian.PutUint32(fixed[28:], e.Uid)
		binary.BigEndian.PutUint32(fixed[32:], e.Gid)
		binary.BigEndian.PutUint32(fixed[36:], e.Size)
		copy(fixed[40:60], e.OID[:])
		binary.BigEndian.PutUint16(fixed[60:], makeFlags(e.Path))
		buf.Write(fixed[:])
		buf.WriteString(e.Path)

		entryLen := 62 + len(e.Path)
		padded := (entryLen + 8) &^ 7
		pad := padded - entryLen
		buf.Write(make([]byte, pad))
	}

	sum := sha1.Sum(buf.Bytes())
	buf.Write(sum[:])
	return buf.Bytes()
}

// Write atomically writes the index to path.
func (idx *Index) Write(path string) error {
	data := idx.Bytes()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "index_*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}
