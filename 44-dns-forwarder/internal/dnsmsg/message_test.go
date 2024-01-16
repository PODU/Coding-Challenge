package dnsmsg

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

// buildQuery returns the wire bytes of a standard recursive A query for
// www.google.com, built by hand from RFC 1035 fields.
func buildQuery() []byte {
	var b []byte
	hdr := make([]byte, 12)
	binary.BigEndian.PutUint16(hdr[0:2], 0x1234) // ID
	binary.BigEndian.PutUint16(hdr[2:4], 0x0100) // RD=1
	binary.BigEndian.PutUint16(hdr[4:6], 1)      // QDCOUNT
	b = append(b, hdr...)
	// QNAME www.google.com
	b = append(b, 3, 'w', 'w', 'w', 6, 'g', 'o', 'o', 'g', 'l', 'e', 3, 'c', 'o', 'm', 0)
	b = append(b, 0x00, 0x01) // QTYPE A
	b = append(b, 0x00, 0x01) // QCLASS IN
	return b
}

func TestDecodeQuery(t *testing.T) {
	m, err := Decode(buildQuery())
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if m.Header.ID != 0x1234 {
		t.Errorf("ID = %#x, want 0x1234", m.Header.ID)
	}
	if !m.Header.RD || m.Header.QR {
		t.Errorf("flags: RD=%v QR=%v, want RD=true QR=false", m.Header.RD, m.Header.QR)
	}
	if m.Header.QDCount != 1 || len(m.Questions) != 1 {
		t.Fatalf("QDCount=%d questions=%d, want 1/1", m.Header.QDCount, len(m.Questions))
	}
	q := m.Questions[0]
	if q.Name != "www.google.com" {
		t.Errorf("name = %q, want www.google.com", q.Name)
	}
	if q.Type != TypeA || q.Class != ClassIN {
		t.Errorf("type/class = %d/%d, want %d/%d", q.Type, q.Class, TypeA, ClassIN)
	}
}

func TestRoundTripQuery(t *testing.T) {
	orig := buildQuery()
	m, err := Decode(orig)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	got, err := Encode(m)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !bytes.Equal(orig, got) {
		t.Errorf("round-trip mismatch:\n orig=%x\n got =%x", orig, got)
	}
}

// buildResponseWithCompression returns a response to the www.google.com query
// where the answer's name is a 0xC0 compression pointer back to the question.
func buildResponseWithCompression() []byte {
	var b []byte
	hdr := make([]byte, 12)
	binary.BigEndian.PutUint16(hdr[0:2], 0x1234) // ID
	binary.BigEndian.PutUint16(hdr[2:4], 0x8180) // QR=1 RD=1 RA=1
	binary.BigEndian.PutUint16(hdr[4:6], 1)      // QDCOUNT
	binary.BigEndian.PutUint16(hdr[6:8], 1)      // ANCOUNT
	b = append(b, hdr...)
	// Question at offset 12.
	b = append(b, 3, 'w', 'w', 'w', 6, 'g', 'o', 'o', 'g', 'l', 'e', 3, 'c', 'o', 'm', 0)
	b = append(b, 0x00, 0x01, 0x00, 0x01)
	// Answer: name = pointer to offset 12.
	b = append(b, 0xC0, 0x0C)
	b = append(b, 0x00, 0x01)             // TYPE A
	b = append(b, 0x00, 0x01)             // CLASS IN
	b = append(b, 0x00, 0x00, 0x01, 0x2C) // TTL = 300
	b = append(b, 0x00, 0x04)             // RDLENGTH
	b = append(b, 8, 8, 8, 8)             // 8.8.8.8
	return b
}

func TestDecodeResponseCompression(t *testing.T) {
	m, err := Decode(buildResponseWithCompression())
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !m.Header.QR || !m.Header.RA {
		t.Errorf("expected QR and RA set")
	}
	if len(m.Answers) != 1 {
		t.Fatalf("answers = %d, want 1", len(m.Answers))
	}
	a := m.Answers[0]
	if a.Name != "www.google.com" {
		t.Errorf("answer name = %q (compression not followed)", a.Name)
	}
	if a.TTL != 300 {
		t.Errorf("TTL = %d, want 300", a.TTL)
	}
	if !bytes.Equal(a.RData, []byte{8, 8, 8, 8}) {
		t.Errorf("rdata = %v, want 8.8.8.8", a.RData)
	}
	if got, ok := m.MinAnswerTTL(); !ok || got != 300 {
		t.Errorf("MinAnswerTTL = %d,%v, want 300,true", got, ok)
	}
}

func TestDecodeMalformed(t *testing.T) {
	cases := map[string][]byte{
		"too short":       {0x12, 0x34, 0x01},
		"truncated qname": append(buildQuery()[:12], 5, 'a', 'b'), // claims 5-byte label, only 2 present
		"qdcount no body": {0, 0, 0x01, 0, 0, 1, 0, 0, 0, 0, 0, 0},
		"bad label type":  append(buildQuery()[:12], 0x80, 0x00),
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Decode(in); err == nil {
				t.Errorf("expected error, got nil")
			}
		})
	}
}

func TestDecodePointerLoop(t *testing.T) {
	// Header + a name at offset 12 that points to itself.
	b := make([]byte, 12)
	binary.BigEndian.PutUint16(b[4:6], 1) // QDCOUNT
	b = append(b, 0xC0, 0x0C)             // pointer -> offset 12 (itself)
	b = append(b, 0x00, 0x01, 0x00, 0x01)
	if _, err := Decode(b); err == nil {
		t.Fatalf("expected pointer-loop error, got nil")
	} else if !errors.Is(err, ErrPointerLoop) && !errors.Is(err, ErrBadPointer) {
		t.Logf("got error %v (acceptable: any non-nil)", err)
	}
}

func TestEncodeDecodeAnswerRoundTrip(t *testing.T) {
	m := &Message{
		Header: Header{ID: 0xABCD, QR: true, RD: true, RA: true, RCode: RCodeNoError},
		Questions: []Question{
			{Name: "example.com", Type: TypeA, Class: ClassIN},
		},
		Answers: []ResourceRecord{
			{Name: "example.com", Type: TypeA, Class: ClassIN, TTL: 600, RData: []byte{93, 184, 216, 34}},
		},
	}
	raw, err := Encode(m)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got.Header.ID != 0xABCD || !got.Header.QR {
		t.Errorf("header mismatch: %+v", got.Header)
	}
	if len(got.Answers) != 1 || got.Answers[0].TTL != 600 {
		t.Fatalf("answer mismatch: %+v", got.Answers)
	}
	if got.Answers[0].Name != "example.com" {
		t.Errorf("answer name = %q", got.Answers[0].Name)
	}
	if !bytes.Equal(got.Answers[0].RData, []byte{93, 184, 216, 34}) {
		t.Errorf("rdata = %v", got.Answers[0].RData)
	}
}

func TestRewriteTTL(t *testing.T) {
	raw := buildResponseWithCompression()
	if err := RewriteTTL(raw, 42); err != nil {
		t.Fatalf("RewriteTTL: %v", err)
	}
	m, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if m.Answers[0].TTL != 42 {
		t.Errorf("TTL after rewrite = %d, want 42", m.Answers[0].TTL)
	}
	// RDATA must be untouched.
	if !bytes.Equal(m.Answers[0].RData, []byte{8, 8, 8, 8}) {
		t.Errorf("rdata corrupted: %v", m.Answers[0].RData)
	}
}

func TestBuildResponseFormErr(t *testing.T) {
	out := BuildResponse(buildQuery(), RCodeFormErr)
	m, err := Decode(out)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !m.Header.QR || m.Header.RCode != RCodeFormErr {
		t.Errorf("QR=%v RCode=%d, want QR=true RCode=%d", m.Header.QR, m.Header.RCode, RCodeFormErr)
	}
	if m.Header.ID != 0x1234 {
		t.Errorf("ID = %#x, want echoed 0x1234", m.Header.ID)
	}
	if len(m.Questions) != 1 || m.Questions[0].Name != "www.google.com" {
		t.Errorf("question not echoed: %+v", m.Questions)
	}
}

func TestBuildResponseGarbage(t *testing.T) {
	// Must not panic and must produce a valid 12-byte header even for junk.
	out := BuildResponse([]byte{0xAA}, RCodeServFail)
	if len(out) < 12 {
		t.Fatalf("output too short: %d", len(out))
	}
	m, err := Decode(out)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if m.Header.RCode != RCodeServFail || !m.Header.QR {
		t.Errorf("bad header: %+v", m.Header)
	}
}

func TestSetIDAndTruncated(t *testing.T) {
	buf := buildQuery()
	SetID(buf, 0xBEEF)
	if binary.BigEndian.Uint16(buf[0:2]) != 0xBEEF {
		t.Errorf("SetID failed: %#x", binary.BigEndian.Uint16(buf[0:2]))
	}
	SetID([]byte{0x01}, 0x1234) // must not panic on short input

	if Truncated(buf) {
		t.Error("query should not be marked truncated")
	}
	binary.BigEndian.PutUint16(buf[2:4], 0x0200) // set TC bit
	if !Truncated(buf) {
		t.Error("TC bit not detected")
	}
	if Truncated([]byte{0x00}) {
		t.Error("short buffer should report not truncated")
	}
}

func TestEncodeLabelTooLong(t *testing.T) {
	long := make([]byte, 64)
	for i := range long {
		long[i] = 'a'
	}
	m := &Message{Questions: []Question{{Name: string(long), Type: TypeA, Class: ClassIN}}}
	if _, err := Encode(m); !errors.Is(err, ErrLabelLong) {
		t.Errorf("err = %v, want ErrLabelLong", err)
	}
}

func TestDecodeTruncatedAnswer(t *testing.T) {
	// Valid header+question claiming one answer, but the answer is cut off.
	b := buildResponseWithCompression()
	if _, err := Decode(b[:len(b)-6]); err == nil {
		t.Error("expected truncation error on cut answer")
	}
}

func TestMinAnswerTTLSkipsOPT(t *testing.T) {
	m := &Message{
		Answers: []ResourceRecord{
			{Type: TypeOPT, TTL: 0},
			{Type: TypeA, TTL: 120},
		},
	}
	if ttl, ok := m.MinAnswerTTL(); !ok || ttl != 120 {
		t.Errorf("MinAnswerTTL = %d,%v, want 120,true", ttl, ok)
	}
	empty := &Message{}
	if _, ok := empty.MinAnswerTTL(); ok {
		t.Error("empty answers should report ok=false")
	}
}
