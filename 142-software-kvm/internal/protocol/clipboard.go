package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
)

// Clipboard identifiers and formats (Barrier's ClipboardID and
// IClipboard::EFormat).
const (
	ClipboardMain      uint8 = 0 // the normal clipboard
	ClipboardSelection uint8 = 1 // X11 primary selection
	NumClipboards            = 2

	FormatText   uint32 = 0 // UTF-8, newlines as \n
	FormatHTML   uint32 = 1
	FormatBitmap uint32 = 2
)

// Chunk marks used in DCLP (Barrier's EDataTransfer).
const (
	MarkStart uint8 = 1 // data = total size as a decimal string
	MarkChunk uint8 = 2 // data = the next slice of the marshalled clipboard
	MarkEnd   uint8 = 3 // data = ""
)

// ChunkSize is how much clipboard data goes in each DCLP chunk. The
// receiver just appends, so any size below MaxMessageSize interoperates.
const ChunkSize = 32 * 1024

// MarshalClipboard encodes formats the way IClipboard::marshall does:
// a 4-byte format count, then per format a 4-byte id, 4-byte size and the
// data, all big-endian. Formats are written in id order.
func MarshalClipboard(formats map[uint32]string) string {
	var b []byte
	b = binary.BigEndian.AppendUint32(b, uint32(len(formats)))
	for _, id := range slices.Sorted(maps.Keys(formats)) {
		d := formats[id]
		b = binary.BigEndian.AppendUint32(b, id)
		b = binary.BigEndian.AppendUint32(b, uint32(len(d)))
		b = append(b, d...)
	}
	return string(b)
}

// TextClipboard marshals a clipboard holding only text.
func TextClipboard(text string) string {
	return MarshalClipboard(map[uint32]string{FormatText: text})
}

// UnmarshalClipboard decodes IClipboard::marshall output.
func UnmarshalClipboard(data string) (map[uint32]string, error) {
	b := []byte(data)
	if len(b) < 4 {
		return nil, errors.New("clipboard: missing format count")
	}
	n := binary.BigEndian.Uint32(b)
	b = b[4:]
	out := make(map[uint32]string)
	for range n {
		if len(b) < 8 {
			return nil, errors.New("clipboard: truncated format header")
		}
		id := binary.BigEndian.Uint32(b)
		size := binary.BigEndian.Uint32(b[4:])
		b = b[8:]
		if uint64(size) > uint64(len(b)) {
			return nil, errors.New("clipboard: truncated format data")
		}
		out[id] = string(b[:size])
		b = b[size:]
	}
	return out, nil
}

// ChunkClipboard splits marshalled clipboard data into the DCLP sequence
// start, chunk..., end.
func ChunkClipboard(id uint8, seq uint32, data string) []ClipboardData {
	msgs := []ClipboardData{{ID: id, Seq: seq, Mark: MarkStart, Data: strconv.Itoa(len(data))}}
	for off := 0; off < len(data); off += ChunkSize {
		end := min(off+ChunkSize, len(data))
		msgs = append(msgs, ClipboardData{ID: id, Seq: seq, Mark: MarkChunk, Data: data[off:end]})
	}
	return append(msgs, ClipboardData{ID: id, Seq: seq, Mark: MarkEnd})
}

// Assembler rebuilds clipboard transfers from DCLP chunks, one transfer per
// clipboard id at a time.
type Assembler struct {
	parts [NumClipboards]*partial
}

type partial struct {
	want int
	buf  []byte
}

// Add consumes one chunk. When the end chunk arrives it returns the full
// marshalled data and done = true.
func (a *Assembler) Add(m ClipboardData) (data string, done bool, err error) {
	if int(m.ID) >= NumClipboards {
		return "", false, fmt.Errorf("clipboard: bad id %d", m.ID)
	}
	p := &a.parts[m.ID]
	switch m.Mark {
	case MarkStart:
		n, err := strconv.Atoi(m.Data)
		if err != nil || n < 0 || n > MaxMessageSize*64 {
			return "", false, fmt.Errorf("clipboard: bad size %q", m.Data)
		}
		*p = &partial{want: n, buf: make([]byte, 0, min(n, MaxMessageSize))}
	case MarkChunk:
		if *p == nil {
			return "", false, errors.New("clipboard: chunk without start")
		}
		(*p).buf = append((*p).buf, m.Data...)
		if len((*p).buf) > (*p).want {
			*p = nil
			return "", false, errors.New("clipboard: more data than announced")
		}
	case MarkEnd:
		if *p == nil {
			return "", false, errors.New("clipboard: end without start")
		}
		got, want := (*p).buf, (*p).want
		*p = nil
		if len(got) != want {
			return "", false, fmt.Errorf("clipboard: got %d bytes, expected %d", len(got), want)
		}
		return string(got), true, nil
	default:
		return "", false, fmt.Errorf("clipboard: bad mark %d", m.Mark)
	}
	return "", false, nil
}
