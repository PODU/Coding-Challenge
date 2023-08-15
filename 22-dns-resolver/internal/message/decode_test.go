package message

import (
	"encoding/hex"
	"testing"
)

func decodeHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hex fixture: %v", err)
	}
	return b
}

// googleResponse is a hand-built A response for dns.google.com with two answer
// records whose owner NAME is a compression pointer (0xC00C) back to the
// question at offset 12.
const googleResponse = "" +
	"1234" + "8180" + "0001" + "0002" + "0000" + "0000" + // header: QR,RD,RA; AN=2
	"03646e7306676f6f676c6503636f6d00" + "0001" + "0001" + // Q: dns.google.com A IN
	"c00c" + "0001" + "0001" + "00000258" + "0004" + "08080808" + // A 8.8.8.8 ttl 600
	"c00c" + "0001" + "0001" + "00000258" + "0004" + "08080404" //  A 8.8.4.4 ttl 600

func TestUnpackGoogleResponse(t *testing.T) {
	buf := decodeHex(t, googleResponse)
	msg, err := Unpack(buf)
	if err != nil {
		t.Fatalf("Unpack: %v", err)
	}
	if msg.Header.ID != 0x1234 {
		t.Errorf("ID = %#x, want 0x1234", msg.Header.ID)
	}
	if !msg.Header.QR() {
		t.Error("QR not set")
	}
	if msg.Header.RCode() != RCodeNoError {
		t.Errorf("RCODE = %d, want 0", msg.Header.RCode())
	}
	if len(msg.Questions) != 1 || msg.Questions[0].Name != "dns.google.com" {
		t.Fatalf("question = %+v", msg.Questions)
	}
	if len(msg.Answers) != 2 {
		t.Fatalf("answers = %d, want 2", len(msg.Answers))
	}

	wantIPs := []string{"8.8.8.8", "8.8.4.4"}
	for i, rec := range msg.Answers {
		// NAME was a compression pointer; must reconstruct to the question.
		if rec.Name != "dns.google.com" {
			t.Errorf("answer[%d].Name = %q, want dns.google.com", i, rec.Name)
		}
		if rec.Type != TypeA || rec.Class != ClassIN {
			t.Errorf("answer[%d] type/class = %d/%d", i, rec.Type, rec.Class)
		}
		if rec.TTL != 600 {
			t.Errorf("answer[%d].TTL = %d, want 600", i, rec.TTL)
		}
		addr, ok := rec.AString()
		if !ok || addr.String() != wantIPs[i] {
			t.Errorf("answer[%d] A = %v (ok=%v), want %s", i, addr, ok, wantIPs[i])
		}
	}
}

// referralResponse is a hand-built NS referral exercising compression pointers
// in both a record owner name and inside NS RData, plus a glue A record.
//
//	Q:   com NS IN                                  (name at offset 12)
//	NS:  owner -> ptr(12); RData = "a" + ptr(12)  => a.com   (RData at 33)
//	A:   owner -> ptr(33) => a.com; RData 1.2.3.4
const referralResponse = "" +
	"abcd" + "8000" + "0001" + "0000" + "0001" + "0001" + // QR; NS=1, AR=1
	"03636f6d00" + "0002" + "0001" + // Q: com NS IN
	"c00c" + "0002" + "0001" + "00002a30" + "0004" + "0161c00c" + // NS a.com
	"c021" + "0001" + "0001" + "00002a30" + "0004" + "01020304" //   A a.com 1.2.3.4

func TestUnpackReferralCompression(t *testing.T) {
	buf := decodeHex(t, referralResponse)
	msg, err := Unpack(buf)
	if err != nil {
		t.Fatalf("Unpack: %v", err)
	}
	if len(msg.Authority) != 1 {
		t.Fatalf("authority = %d, want 1", len(msg.Authority))
	}
	ns, ok := msg.Authority[0].NSString(buf)
	if !ok || ns != "a.com" {
		t.Errorf("NSString = %q (ok=%v), want a.com", ns, ok)
	}
	if len(msg.Additional) != 1 {
		t.Fatalf("additional = %d, want 1", len(msg.Additional))
	}
	if msg.Additional[0].Name != "a.com" {
		t.Errorf("glue name = %q, want a.com", msg.Additional[0].Name)
	}
	addr, ok := msg.Additional[0].AString()
	if !ok || addr.String() != "1.2.3.4" {
		t.Errorf("glue A = %v (ok=%v), want 1.2.3.4", addr, ok)
	}
}

func TestUnpackCNAMECompression(t *testing.T) {
	// Q: www.example.com A; Answer: CNAME -> "example.com" via "example"+ptr.
	// Lay out so that "com" can be pointed at.
	//  offset 12: 03 777777 07 6578616d706c65 03 636f6d 00  (www.example.com)
	//   www at 13, example at 17(len)+...; com label starts at offset 28.
	// Compute: 12:03 13:77 14:77 15:77 16:07 17..23 example 24:03 25:63 26:6f 27:6d 28:00
	// "com" label is at offset 24. Pointer to com = 0xC018 (24).
	const fixture = "" +
		"0001" + "8180" + "0001" + "0001" + "0000" + "0000" +
		"03777777076578616d706c6503636f6d00" + "0001" + "0001" +
		// CNAME record: owner ptr(12)=www.example.com; RData "example"+ptr(24)
		"c00c" + "0005" + "0001" + "00000e10" + "0009" + "076578616d706c65c018"
	buf := decodeHex(t, fixture)
	msg, err := Unpack(buf)
	if err != nil {
		t.Fatalf("Unpack: %v", err)
	}
	if len(msg.Answers) != 1 || msg.Answers[0].Type != TypeCNAME {
		t.Fatalf("answers = %+v", msg.Answers)
	}
	cn, ok := msg.Answers[0].CNAMEString(buf)
	if !ok || cn != "example.com" {
		t.Errorf("CNAMEString = %q (ok=%v), want example.com", cn, ok)
	}
}

func TestUnpackMaliciousInputs(t *testing.T) {
	tests := []struct {
		name string
		hex  string
	}{
		{"zero buffer", ""},
		{"short header", "1234"},
		// QDCOUNT=1 but name is a pointer to itself at offset 12 -> loop.
		{"pointer loop", "000000000001000000000000" + "c00c"},
		// Pointer target 0x00ff far past end of buffer.
		{"pointer past end", "000000000001000000000000" + "c0ff"},
		// QDCOUNT=1 valid question, ANCOUNT=1 but no record bytes follow.
		{"truncated record", "000000000001000100000000" + "03636f6d00" + "0001" + "0001"},
		// RDLENGTH claims 16 bytes that are not present.
		{"rdata past end", "000000000001000100000000" + "03636f6d00" + "0001" + "0001" +
			"c00c" + "0001" + "0001" + "00000258" + "0010" + "08080808"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := decodeHex(t, tt.hex)
			// Must not panic; must return an error.
			msg, err := Unpack(buf)
			if err == nil {
				t.Errorf("expected error, got message %+v", msg)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	orig := NewQuery(0x4242, "dns.google.com", TypeA, ClassIN, true)
	packed, err := orig.Pack()
	if err != nil {
		t.Fatal(err)
	}
	got, err := Unpack(packed)
	if err != nil {
		t.Fatalf("Unpack(Pack()): %v", err)
	}
	if got.Header.ID != orig.Header.ID {
		t.Errorf("ID: got %#x want %#x", got.Header.ID, orig.Header.ID)
	}
	if !got.Header.RD() {
		t.Error("RD bit lost in round-trip")
	}
	if got.Header.QDCount != 1 || len(got.Questions) != 1 {
		t.Fatalf("question count wrong: %+v", got.Header)
	}
	q := got.Questions[0]
	if q.Name != "dns.google.com" || q.Type != TypeA || q.Class != ClassIN {
		t.Errorf("question round-trip = %+v", q)
	}
}
