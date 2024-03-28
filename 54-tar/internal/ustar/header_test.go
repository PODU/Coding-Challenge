package ustar

import (
	"bytes"
	"testing"
	"time"
)

func TestOctalRoundTrip(t *testing.T) {
	cases := []struct {
		v     int64
		width int
	}{
		{0, 8}, {1, 8}, {0o755, 8}, {0o7777, 8},
		{512, 12}, {1<<21 - 1, 8}, {1234567, 12},
	}
	for _, c := range cases {
		enc, err := encodeOctal(c.v, c.width)
		if err != nil {
			t.Fatalf("encodeOctal(%d,%d): %v", c.v, c.width, err)
		}
		if len(enc) != c.width {
			t.Fatalf("width = %d, want %d", len(enc), c.width)
		}
		if enc[c.width-1] != 0 {
			t.Errorf("field not NUL-terminated for %d", c.v)
		}
		got, err := parseOctal(enc)
		if err != nil {
			t.Fatalf("parseOctal: %v", err)
		}
		if got != c.v {
			t.Errorf("round trip %d -> %d", c.v, got)
		}
	}
}

func TestEncodeOctalOverflow(t *testing.T) {
	if _, err := encodeOctal(0o7777777, 4); err == nil {
		t.Error("expected overflow error for value exceeding width")
	}
	if _, err := encodeOctal(-1, 8); err == nil {
		t.Error("expected error for negative value")
	}
}

func TestParseOctalEdge(t *testing.T) {
	// space/NUL padded and empty
	if v, _ := parseOctal([]byte("000644\x00 ")); v != 0o644 {
		t.Errorf("got %o want 644", v)
	}
	if v, _ := parseOctal([]byte("\x00\x00\x00\x00")); v != 0 {
		t.Errorf("empty field should be 0, got %d", v)
	}
	if v, _ := parseOctal([]byte("   755 ")); v != 0o755 {
		t.Errorf("got %o want 755", v)
	}
}

func TestParseOctalRejectsBase256(t *testing.T) {
	field := []byte{0x80, 0, 0, 0, 0, 0, 0, 0}
	if _, err := parseOctal(field); err == nil {
		t.Error("expected base-256 rejection")
	}
}

func TestMarshalKnownOffsets(t *testing.T) {
	h := &Header{
		Name:     "hello.txt",
		Mode:     0o644,
		UID:      0,
		GID:      0,
		Size:     5,
		ModTime:  time.Unix(0, 0),
		Typeflag: TypeReg,
		Uname:    "root",
		Gname:    "root",
	}
	block, err := h.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	b := block[:]

	if got := getString(b[offName : offName+sizeName]); got != "hello.txt" {
		t.Errorf("name = %q", got)
	}
	if !bytes.Equal(b[offMagic:offMagic+sizeMagic], []byte(Magic)) {
		t.Errorf("magic = %q", b[offMagic:offMagic+sizeMagic])
	}
	if !bytes.Equal(b[offVersion:offVersion+sizeVersion], []byte(Version)) {
		t.Errorf("version = %q", b[offVersion:offVersion+sizeVersion])
	}
	if b[offTypeflag] != TypeReg {
		t.Errorf("typeflag = %q", b[offTypeflag])
	}
	// mode "0000644\0"
	if got := string(b[offMode : offMode+sizeMode]); got != "0000644\x00" {
		t.Errorf("mode field = %q", got)
	}
	// size 5 -> "00000000005\0"
	if got := string(b[offSize : offSize+sizeSize]); got != "00000000005\x00" {
		t.Errorf("size field = %q", got)
	}
	// checksum field ends with NUL then space
	cf := b[offChksum : offChksum+sizeChksum]
	if cf[6] != 0 || cf[7] != ' ' {
		t.Errorf("checksum terminator = %v", cf[6:])
	}
	if !verifyChecksum(b) {
		t.Error("checksum did not verify")
	}
}

func TestChecksumValueIsCorrect(t *testing.T) {
	h := &Header{Name: "a", Mode: 0o644, Size: 0, ModTime: time.Unix(0, 0), Typeflag: TypeReg, Uname: "root", Gname: "root"}
	block, _ := h.Marshal()
	stored, err := parseOctal(block[offChksum : offChksum+sizeChksum])
	if err != nil {
		t.Fatal(err)
	}
	unsigned, _ := computeChecksum(block[:])
	if stored != unsigned {
		t.Errorf("stored checksum %o != recomputed %o", stored, unsigned)
	}
}

func TestParseHeaderRoundTrip(t *testing.T) {
	orig := &Header{
		Name:     "dir/subdir/file.txt",
		Mode:     0o755,
		UID:      1000,
		GID:      1000,
		Size:     42,
		ModTime:  time.Unix(1700000000, 0),
		Typeflag: TypeReg,
		Linkname: "",
		Uname:    "alice",
		Gname:    "users",
	}
	block, err := orig.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseHeader(block[:])
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != orig.Name || got.Mode != orig.Mode || got.UID != orig.UID ||
		got.GID != orig.GID || got.Size != orig.Size || got.Typeflag != orig.Typeflag ||
		got.Uname != orig.Uname || got.Gname != orig.Gname {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, orig)
	}
	if !got.ModTime.Equal(orig.ModTime) {
		t.Errorf("mtime %v != %v", got.ModTime, orig.ModTime)
	}
}

func TestChecksumTamperDetected(t *testing.T) {
	h := &Header{Name: "x", Mode: 0o644, ModTime: time.Unix(0, 0), Typeflag: TypeReg}
	block, _ := h.Marshal()
	block[0] ^= 0xFF // corrupt the name byte
	if _, err := ParseHeader(block[:]); err == nil {
		t.Error("expected checksum mismatch error")
	}
}

func TestSignedChecksumAccepted(t *testing.T) {
	// Build a header then rewrite its checksum field using the signed sum, to
	// emulate a legacy writer; the reader must still accept it.
	h := &Header{Name: "y", Mode: 0o644, ModTime: time.Unix(0, 0), Typeflag: TypeReg}
	block, _ := h.Marshal()
	_, signed := computeChecksum(block[:])
	enc, _ := encodeOctal(signed, 7)
	field := block[offChksum : offChksum+sizeChksum]
	for i := range field {
		field[i] = 0
	}
	copy(field, enc)
	field[7] = ' '
	if !verifyChecksum(block[:]) {
		t.Error("signed checksum should be accepted")
	}
}

func TestZeroBlockDetected(t *testing.T) {
	var zero [BlockSize]byte
	if _, err := ParseHeader(zero[:]); err != ErrZeroBlock {
		t.Errorf("expected ErrZeroBlock, got %v", err)
	}
}

func TestPrefixSplitLongName(t *testing.T) {
	// 90-char dir + "/" + 30-char file = 121 chars: must split into prefix/name.
	dir := string(bytes.Repeat([]byte("a"), 90))
	file := string(bytes.Repeat([]byte("b"), 30))
	name := dir + "/" + file
	if !FitsUSTARName(name) {
		t.Fatal("expected name to fit via prefix split")
	}
	h := &Header{Name: name, Mode: 0o644, ModTime: time.Unix(0, 0), Typeflag: TypeReg}
	block, err := h.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseHeader(block[:])
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != name {
		t.Errorf("reconstructed name %q != %q", got.Name, name)
	}
	// prefix field should be non-empty
	if getString(block[offPrefix:offPrefix+sizePrefix]) == "" {
		t.Error("expected prefix field to be used")
	}
}

func TestVeryLongNameDoesNotFit(t *testing.T) {
	// A single 200-char component (no '/') cannot be split.
	name := string(bytes.Repeat([]byte("z"), 200))
	if FitsUSTARName(name) {
		t.Error("expected name not to fit USTAR fields")
	}
}
