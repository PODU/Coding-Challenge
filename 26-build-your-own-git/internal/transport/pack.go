// Package transport implements packfile construction/parsing and the smart-HTTP
// (git-receive-pack) push protocol.
package transport

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"io"

	"gg/internal/object"
)

// Pack object type codes.
const (
	objCommit = 1
	objTree   = 2
	objBlob   = 3
	objTag    = 4
)

func typeCode(t object.Type) (byte, error) {
	switch t {
	case object.TypeCommit:
		return objCommit, nil
	case object.TypeTree:
		return objTree, nil
	case object.TypeBlob:
		return objBlob, nil
	case object.TypeTag:
		return objTag, nil
	}
	return 0, fmt.Errorf("unpackable object type %q", t)
}

func codeType(c byte) (object.Type, error) {
	switch c {
	case objCommit:
		return object.TypeCommit, nil
	case objTree:
		return object.TypeTree, nil
	case objBlob:
		return object.TypeBlob, nil
	case objTag:
		return object.TypeTag, nil
	}
	return "", fmt.Errorf("unknown pack object type %d", c)
}

// PackObject is one undeltified object to place in a pack.
type PackObject struct {
	Type    object.Type
	Payload []byte
}

// writeObjHeader writes the varint type+size header for a pack object.
func writeObjHeader(buf *bytes.Buffer, code byte, size int) {
	// First byte: MSB continuation, bits 6-4 type, bits 3-0 low size.
	b := byte(code<<4) | byte(size&0x0f)
	size >>= 4
	for size > 0 {
		buf.WriteByte(b | 0x80)
		b = byte(size & 0x7f)
		size >>= 7
	}
	buf.WriteByte(b)
}

// BuildPack serializes objects into a version-2 packfile (no deltas).
func BuildPack(objs []PackObject) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("PACK")
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], 2)
	buf.Write(hdr[:])
	binary.BigEndian.PutUint32(hdr[:], uint32(len(objs)))
	buf.Write(hdr[:])

	for _, o := range objs {
		code, err := typeCode(o.Type)
		if err != nil {
			return nil, err
		}
		writeObjHeader(&buf, code, len(o.Payload))

		zw := zlib.NewWriter(&buf)
		if _, err := zw.Write(o.Payload); err != nil {
			return nil, err
		}
		if err := zw.Close(); err != nil {
			return nil, err
		}
	}

	sum := sha1.Sum(buf.Bytes())
	buf.Write(sum[:])
	return buf.Bytes(), nil
}

// ParsePack parses a packfile, returning its objects and verifying the trailer.
func ParsePack(data []byte) ([]PackObject, error) {
	if len(data) < 12+20 {
		return nil, fmt.Errorf("pack too short")
	}
	if string(data[:4]) != "PACK" {
		return nil, fmt.Errorf("bad pack signature")
	}
	ver := binary.BigEndian.Uint32(data[4:8])
	if ver != 2 {
		return nil, fmt.Errorf("unsupported pack version %d", ver)
	}
	count := binary.BigEndian.Uint32(data[8:12])

	body := data[:len(data)-20]
	want := data[len(data)-20:]
	got := sha1.Sum(body)
	if !bytes.Equal(got[:], want) {
		return nil, fmt.Errorf("pack checksum mismatch")
	}

	r := bytes.NewReader(body[12:]) // bytes.Reader is an io.ByteReader: flate won't over-read
	var objs []PackObject
	for i := uint32(0); i < count; i++ {
		first, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		code := (first >> 4) & 0x07
		size := uint64(first & 0x0f)
		shift := uint(4)
		for first&0x80 != 0 {
			first, err = r.ReadByte()
			if err != nil {
				return nil, err
			}
			size |= uint64(first&0x7f) << shift
			shift += 7
		}

		typ, err := codeType(code)
		if err != nil {
			return nil, err
		}

		zr, err := zlib.NewReader(r)
		if err != nil {
			return nil, fmt.Errorf("object %d: %w", i, err)
		}
		payload, err := io.ReadAll(zr)
		zr.Close()
		if err != nil {
			return nil, fmt.Errorf("object %d: %w", i, err)
		}
		if uint64(len(payload)) != size {
			return nil, fmt.Errorf("object %d: size mismatch (hdr %d, got %d)", i, size, len(payload))
		}
		objs = append(objs, PackObject{Type: typ, Payload: payload})
	}
	return objs, nil
}
