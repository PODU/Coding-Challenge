package peer

import (
	"bytes"
	"reflect"
	"testing"
)

func TestHandshakeRoundTrip(t *testing.T) {
	var ih, id [20]byte
	copy(ih[:], "infohashinfohashinfo")
	copy(id[:], "-GO0001-abcdefghij01")

	hs := NewHandshake(ih, id)
	ser := hs.Serialize()
	if len(ser) != 68 {
		t.Fatalf("handshake serialized to %d bytes, want 68", len(ser))
	}

	got, err := ReadHandshake(bytes.NewReader(ser))
	if err != nil {
		t.Fatal(err)
	}
	if got.Pstr != Pstr {
		t.Errorf("pstr = %q", got.Pstr)
	}
	if got.InfoHash != ih {
		t.Errorf("info hash mismatch")
	}
	if got.PeerID != id {
		t.Errorf("peer id mismatch")
	}
}

func TestMessageRoundTrip(t *testing.T) {
	cases := []*Message{
		{ID: MsgChoke},
		{ID: MsgUnchoke},
		{ID: MsgInterested},
		{ID: MsgNotInterested},
		{ID: MsgHave, Payload: []byte{0, 0, 0, 5}},
		{ID: MsgBitfield, Payload: []byte{0xFF, 0x80}},
		{ID: MsgRequest, Payload: make([]byte, 12)},
		{ID: MsgPiece, Payload: append([]byte{0, 0, 0, 1, 0, 0, 0, 0}, []byte("blockdata")...)},
		{ID: MsgCancel, Payload: make([]byte, 12)},
	}
	for _, m := range cases {
		ser := m.Serialize()
		got, err := ReadMessage(bytes.NewReader(ser))
		if err != nil {
			t.Fatalf("ReadMessage(%s): %v", m, err)
		}
		if got.ID != m.ID {
			t.Errorf("id = %d, want %d", got.ID, m.ID)
		}
		if !bytes.Equal(got.Payload, m.Payload) {
			t.Errorf("payload = %v, want %v", got.Payload, m.Payload)
		}
	}
}

func TestKeepAlive(t *testing.T) {
	var m *Message // nil = keep-alive
	ser := m.Serialize()
	if !bytes.Equal(ser, []byte{0, 0, 0, 0}) {
		t.Fatalf("keep-alive serialized to %v", ser)
	}
	got, err := ReadMessage(bytes.NewReader(ser))
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("keep-alive read as %v, want nil", got)
	}
}

func TestFormatAndParseRequest(t *testing.T) {
	m := FormatRequest(7, 16384, 16384)
	if m.ID != MsgRequest {
		t.Fatalf("id = %d", m.ID)
	}
	if len(m.Payload) != 12 {
		t.Fatalf("request payload = %d bytes", len(m.Payload))
	}
}

func TestParseHave(t *testing.T) {
	m := FormatHave(99)
	idx, err := ParseHave(m)
	if err != nil {
		t.Fatal(err)
	}
	if idx != 99 {
		t.Errorf("have index = %d, want 99", idx)
	}
	if _, err := ParseHave(&Message{ID: MsgChoke}); err == nil {
		t.Error("ParseHave on non-have should error")
	}
	if _, err := ParseHave(&Message{ID: MsgHave, Payload: []byte{1, 2}}); err == nil {
		t.Error("ParseHave with short payload should error")
	}
}

func TestParsePiece(t *testing.T) {
	buf := make([]byte, 32)
	block := []byte("0123456789")
	payload := []byte{0, 0, 0, 3, 0, 0, 0, 8} // index 3, begin 8
	payload = append(payload, block...)
	n, err := ParsePiece(3, buf, &Message{ID: MsgPiece, Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	if n != len(block) {
		t.Errorf("copied %d bytes, want %d", n, len(block))
	}
	if !bytes.Equal(buf[8:8+len(block)], block) {
		t.Errorf("block not placed at begin offset")
	}

	// Index mismatch.
	if _, err := ParsePiece(2, buf, &Message{ID: MsgPiece, Payload: payload}); err == nil {
		t.Error("expected index-mismatch error")
	}
	// Out-of-bounds begin.
	bad := []byte{0, 0, 0, 3, 0xFF, 0xFF, 0xFF, 0xFF, 1}
	if _, err := ParsePiece(3, buf, &Message{ID: MsgPiece, Payload: bad}); err == nil {
		t.Error("expected out-of-bounds error")
	}
}

func TestBitfield(t *testing.T) {
	// High bit of byte 0 is piece 0.
	bf := Bitfield{0b10100000, 0b00000001}
	want := map[int]bool{0: true, 1: false, 2: true, 8: false, 15: true}
	for idx, exp := range want {
		if bf.HasPiece(idx) != exp {
			t.Errorf("HasPiece(%d) = %v, want %v", idx, bf.HasPiece(idx), exp)
		}
	}
	// Out of range is false.
	if bf.HasPiece(16) || bf.HasPiece(-1) {
		t.Error("out-of-range HasPiece should be false")
	}

	// SetPiece round-trip.
	bf2 := make(Bitfield, 2)
	bf2.SetPiece(0)
	bf2.SetPiece(9)
	if !reflect.DeepEqual([]byte(bf2), []byte{0b10000000, 0b01000000}) {
		t.Errorf("SetPiece produced %08b", []byte(bf2))
	}
}
