package message

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

func TestEncodeName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []byte
	}{
		{"basic", "dns.google.com", []byte("\x03dns\x06google\x03com\x00")},
		{"trailing dot", "dns.google.com.", []byte("\x03dns\x06google\x03com\x00")},
		{"single label", "com", []byte("\x03com\x00")},
		{"root empty", "", []byte{0}},
		{"root dot", ".", []byte{0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := encodeName(tt.in)
			if err != nil {
				t.Fatalf("encodeName(%q) error: %v", tt.in, err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("encodeName(%q) = % x, want % x", tt.in, got, tt.want)
			}
		})
	}
}

func TestEncodeNameRejectsOversize(t *testing.T) {
	longLabel := strings.Repeat("a", 64)
	if _, err := encodeName(longLabel); err == nil {
		t.Errorf("expected error for 64-byte label, got nil")
	}

	// A name well over 255 bytes (many 63-byte labels).
	var parts []string
	for i := 0; i < 6; i++ {
		parts = append(parts, strings.Repeat("a", 63))
	}
	longName := strings.Join(parts, ".")
	if _, err := encodeName(longName); err == nil {
		t.Errorf("expected error for >255-byte name, got nil")
	}

	if _, err := encodeName("a..b"); err == nil {
		t.Errorf("expected error for empty label, got nil")
	}
}

func TestPackQueryFixture(t *testing.T) {
	// Hand-computed wire form of a standard A query for dns.google.com:
	//   ID=0x1234, flags=0x0100 (RD set), QDCOUNT=1, others 0
	//   QNAME=\x03dns\x06google\x03com\x00, QTYPE=1, QCLASS=1
	q := NewQuery(0x1234, "dns.google.com", TypeA, ClassIN, true)
	got, err := q.Pack()
	if err != nil {
		t.Fatalf("Pack error: %v", err)
	}

	want := mustHex(t, ""+
		"1234"+ // ID
		"0100"+ // flags: RD=1
		"0001"+ // QDCOUNT
		"0000"+ // ANCOUNT
		"0000"+ // NSCOUNT
		"0000"+ // ARCOUNT
		"03646e7306676f6f676c6503636f6d00"+ // dns.google.com
		"0001"+ // QTYPE=A
		"0001") // QCLASS=IN

	if !bytes.Equal(got, want) {
		t.Errorf("Pack mismatch:\n got % x\nwant % x", got, want)
	}
}

func TestPackHeaderCountsDerived(t *testing.T) {
	q := NewQuery(1, "a.com", TypeA, ClassIN, false)
	got, err := q.Pack()
	if err != nil {
		t.Fatal(err)
	}
	// Flags should be 0x0000 when RD is not desired.
	if got[2] != 0x00 || got[3] != 0x00 {
		t.Errorf("flags = %02x%02x, want 0000", got[2], got[3])
	}
	// QDCOUNT must be 1.
	if got[4] != 0x00 || got[5] != 0x01 {
		t.Errorf("QDCOUNT = %02x%02x, want 0001", got[4], got[5])
	}
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hex fixture: %v", err)
	}
	return b
}
