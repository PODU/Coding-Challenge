package bencode

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	cases := []string{
		"i0e",
		"i42e",
		"i-42e",
		"i9223372036854775807e",
		"0:",
		"4:spam",
		"le",
		"l4:spam4:eggse",
		"l6:Coding10:Challengese",
		"de",
		"d3:cow3:moo4:spam4:eggse",
		"d4:spaml1:a1:bee",
		"d8:announce3:url4:infod6:lengthi3e4:name3:foo12:piece lengthi16384e6:pieces20:aaaaaaaaaaaaaaaaaaaaee",
		"li1ei2ei3ee",
		"l0:0:0:e",
	}
	for _, in := range cases {
		v, err := Decode([]byte(in))
		if err != nil {
			t.Fatalf("Decode(%q) error: %v", in, err)
		}
		out, err := Encode(v)
		if err != nil {
			t.Fatalf("Encode after Decode(%q) error: %v", in, err)
		}
		if !bytes.Equal(out, []byte(in)) {
			t.Errorf("round-trip mismatch for %q: got %q", in, out)
		}
	}
}

func TestDecodeTypes(t *testing.T) {
	v, err := Decode([]byte("d3:cow3:moo4:spaml1:a1:be3:numi-7ee"))
	if err != nil {
		t.Fatal(err)
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", v)
	}
	if m["cow"] != "moo" {
		t.Errorf("cow = %v", m["cow"])
	}
	if m["num"] != int64(-7) {
		t.Errorf("num = %v (%T)", m["num"], m["num"])
	}
	spam, ok := m["spam"].([]interface{})
	if !ok || len(spam) != 2 || spam[0] != "a" || spam[1] != "b" {
		t.Errorf("spam = %v", m["spam"])
	}
}

func TestCanonicalKeySorting(t *testing.T) {
	// Keys given out of order must re-encode sorted.
	in := []byte("d4:spam4:eggs3:cow3:mooe")
	v, err := Decode(in)
	if err != nil {
		// Our decoder accepts unsorted input; if it rejected, that's a failure.
		t.Fatalf("decode unsorted dict: %v", err)
	}
	out, err := Encode(v)
	if err != nil {
		t.Fatal(err)
	}
	want := "d3:cow3:moo4:spam4:eggse"
	if string(out) != want {
		t.Errorf("canonical encode = %q, want %q", out, want)
	}
}

func TestDecodeRejectsMalformed(t *testing.T) {
	bad := []string{
		"",                       // empty
		"i-0e",                   // negative zero
		"i03e",                   // leading zero
		"ie",                     // empty integer
		"i e",                    // non-digit
		"i42",                    // unterminated integer
		"5:abc",                  // string too short
		"l1:ae1:b",               // trailing data after list
		"d3:cowe",                // key without value
		"di1e3:fooe",             // non-string key
		"4:spam5",                // trailing garbage
		"d3:cow3:moo3:cow3:bare", // duplicate key
		"-1:a",                   // bad string length
	}
	for _, in := range bad {
		if _, err := Decode([]byte(in)); err == nil {
			t.Errorf("Decode(%q) = nil error, want error", in)
		}
	}
}

func TestBinaryStrings(t *testing.T) {
	binary := []byte{0x00, 0xff, 0x10, 0x80, '\n', ':'}
	encoded, err := Encode(string(binary))
	if err != nil {
		t.Fatal(err)
	}
	v, err := Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if v.(string) != string(binary) {
		t.Errorf("binary string round-trip failed")
	}
}

func TestDecodeWithRawCapturesInfoSpan(t *testing.T) {
	data := []byte("d8:announce3:url4:infod6:lengthi5e4:name1:xee")
	_, raw, err := DecodeWithRaw(data)
	if err != nil {
		t.Fatal(err)
	}
	infoRaw, ok := raw["info"]
	if !ok {
		t.Fatal("no raw info span captured")
	}
	want := "d6:lengthi5e4:name1:xe"
	if string(infoRaw) != want {
		t.Errorf("raw info = %q, want %q", infoRaw, want)
	}
}

func TestSampleTorrentRoundTrip(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "sample.torrent")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	v, err := Decode(data)
	if err != nil {
		t.Fatalf("decode sample.torrent: %v", err)
	}
	out, err := Encode(v)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, data) {
		t.Error("re-encoded sample.torrent is not byte-identical")
	}
}

func TestEncodeByteSlice(t *testing.T) {
	out, err := Encode([]byte{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out, []byte("3:\x01\x02\x03")) {
		t.Errorf("encode []byte = %q", out)
	}
}
