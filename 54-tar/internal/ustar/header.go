package ustar

import (
	"bytes"
	"errors"
	"fmt"
	"time"
)

// ErrZeroBlock signals that a block was all zero bytes (an end-of-archive
// marker block), so it carries no header.
var ErrZeroBlock = errors.New("ustar: zero block")

// Header is a decoded USTAR header. It is filesystem-agnostic: callers in
// higher layers translate between Header and os.FileInfo.
type Header struct {
	Name     string
	Mode     int64
	UID      int
	GID      int
	Size     int64
	ModTime  time.Time
	Typeflag byte
	Linkname string
	Uname    string
	Gname    string
}

// putString copies s into dst (a field slice), NUL-padding the remainder. It
// returns an error if s does not fit (one byte is reserved for the trailing
// NUL only when the value would otherwise fill the whole field — USTAR string
// fields may use the full width without a terminator, matching GNU tar).
func putString(dst []byte, s string) error {
	if len(s) > len(dst) {
		return fmt.Errorf("ustar: value %q exceeds field width %d", s, len(dst))
	}
	copy(dst, s)
	for i := len(s); i < len(dst); i++ {
		dst[i] = 0
	}
	return nil
}

// getString reads a NUL-terminated (or full-width) string field.
func getString(src []byte) string {
	if i := bytes.IndexByte(src, 0); i >= 0 {
		return string(src[:i])
	}
	return string(src)
}

// Marshal renders the header into a single 512-byte block, computing and
// writing the checksum. Long names that do not fit in name+prefix must be
// handled by the caller (via a GNU 'L' record); Marshal itself splits a long
// name across prefix/name when possible and errors if it cannot.
func (h *Header) Marshal() ([BlockSize]byte, error) {
	var block [BlockSize]byte
	b := block[:]

	name, prefix, err := splitName(h.Name)
	if err != nil {
		return block, err
	}

	if err := putString(b[offName:offName+sizeName], name); err != nil {
		return block, err
	}
	if err := putString(b[offPrefix:offPrefix+sizePrefix], prefix); err != nil {
		return block, err
	}
	if err := putString(b[offLinkname:offLinkname+sizeLinkname], h.Linkname); err != nil {
		return block, err
	}

	uname := h.Uname
	if uname == "" {
		uname = "root"
	}
	gname := h.Gname
	if gname == "" {
		gname = "root"
	}
	if err := putString(b[offUname:offUname+sizeUname], uname); err != nil {
		return block, err
	}
	if err := putString(b[offGname:offGname+sizeGname], gname); err != nil {
		return block, err
	}

	type octField struct {
		off, width int
		val        int64
	}
	for _, f := range []octField{
		{offMode, sizeMode, h.Mode & 0o7777},
		{offUID, sizeUID, int64(h.UID)},
		{offGID, sizeGID, int64(h.GID)},
		{offSize, sizeSize, h.Size},
		{offMtime, sizeMtime, h.ModTime.Unix()},
		{offDevmajor, sizeDevmajor, 0},
		{offDevminor, sizeDevminor, 0},
	} {
		enc, err := encodeOctal(f.val, f.width)
		if err != nil {
			return block, err
		}
		copy(b[f.off:f.off+f.width], enc)
	}

	typeflag := h.Typeflag
	if typeflag == 0 {
		typeflag = TypeReg
	}
	b[offTypeflag] = typeflag

	copy(b[offMagic:offMagic+sizeMagic], Magic)
	copy(b[offVersion:offVersion+sizeVersion], Version)

	writeChecksum(b)
	return block, nil
}

// ParseHeader decodes a 512-byte block into a Header. A wholly zero block
// returns ErrZeroBlock. A block whose checksum does not validate returns an
// error.
func ParseHeader(b []byte) (*Header, error) {
	if len(b) != BlockSize {
		return nil, fmt.Errorf("ustar: header block must be %d bytes, got %d", BlockSize, len(b))
	}
	if isZeroBlock(b) {
		return nil, ErrZeroBlock
	}
	if !verifyChecksum(b) {
		return nil, errors.New("ustar: header checksum mismatch (corrupt archive?)")
	}

	h := &Header{}
	name := getString(b[offName : offName+sizeName])
	prefix := getString(b[offPrefix : offPrefix+sizePrefix])
	if prefix != "" {
		h.Name = prefix + "/" + name
	} else {
		h.Name = name
	}
	h.Linkname = getString(b[offLinkname : offLinkname+sizeLinkname])
	h.Uname = getString(b[offUname : offUname+sizeUname])
	h.Gname = getString(b[offGname : offGname+sizeGname])

	var err error
	if h.Mode, err = parseOctal(b[offMode : offMode+sizeMode]); err != nil {
		return nil, err
	}
	var n int64
	if n, err = parseOctal(b[offUID : offUID+sizeUID]); err != nil {
		return nil, err
	}
	h.UID = int(n)
	if n, err = parseOctal(b[offGID : offGID+sizeGID]); err != nil {
		return nil, err
	}
	h.GID = int(n)
	if h.Size, err = parseOctal(b[offSize : offSize+sizeSize]); err != nil {
		return nil, err
	}
	var mtime int64
	if mtime, err = parseOctal(b[offMtime : offMtime+sizeMtime]); err != nil {
		return nil, err
	}
	h.ModTime = time.Unix(mtime, 0)

	h.Typeflag = b[offTypeflag]
	if h.Typeflag == TypeRegA {
		h.Typeflag = TypeReg
	}
	return h, nil
}

// isZeroBlock reports whether every byte in b is zero.
func isZeroBlock(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}
