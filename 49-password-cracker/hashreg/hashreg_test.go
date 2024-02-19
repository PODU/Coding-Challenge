package hashreg

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"math/rand"
	"testing"
)

// Known-answer vectors from the published references.
func TestGoldenVectors(t *testing.T) {
	cases := []struct {
		algo, in, want string
	}{
		{"md5", "abc", "900150983cd24fb0d6963f7d28e17f72"},
		{"md5", "", "d41d8cd98f00b204e9800998ecf8427e"},
		{"sha1", "abc", "a9993e364706816aba3e25717850c26c9cd0d89d"},
		{"sha256", "abc", "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
		{"sha512", "abc", "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f"},
	}
	for _, c := range cases {
		a, err := Get(c.algo)
		if err != nil {
			t.Fatalf("Get(%q): %v", c.algo, err)
		}
		got := HexSum(a, []byte(c.in))
		if got != c.want {
			t.Errorf("%s(%q) = %s, want %s", c.algo, c.in, got, c.want)
		}
		if a.HexLen() != len(c.want) {
			t.Errorf("%s HexLen = %d, want %d", c.algo, a.HexLen(), len(c.want))
		}
	}
}

// Cross-check every registered algo against the stdlib for random inputs.
func TestParityWithStdlib(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	ref := map[string]func([]byte) []byte{
		"md5":    func(d []byte) []byte { s := md5.Sum(d); return s[:] },
		"sha1":   func(d []byte) []byte { s := sha1.Sum(d); return s[:] },
		"sha256": func(d []byte) []byte { s := sha256.Sum256(d); return s[:] },
		"sha512": func(d []byte) []byte { s := sha512.Sum512(d); return s[:] },
	}
	for _, name := range Names() {
		a, _ := Get(name)
		for i := 0; i < 200; i++ {
			buf := make([]byte, r.Intn(64))
			r.Read(buf)
			if hex.EncodeToString(a.Sum(buf)) != hex.EncodeToString(ref[name](buf)) {
				t.Fatalf("%s mismatch vs stdlib for input %x", name, buf)
			}
		}
	}
}

func TestGetUnknown(t *testing.T) {
	if _, err := Get("md4"); err == nil {
		t.Fatal("expected error for unknown algorithm")
	}
	// case-insensitive + trimming
	if _, err := Get("  SHA256 "); err != nil {
		t.Fatalf("expected SHA256 to resolve, got %v", err)
	}
}

func TestDetect(t *testing.T) {
	a, err := Detect(32)
	if err != nil || a.Name() != "md5" {
		t.Fatalf("Detect(32) = %v, %v; want md5", a, err)
	}
	if _, err := Detect(40); err != nil {
		t.Fatalf("Detect(40) should be sha1: %v", err)
	}
	if _, err := Detect(99); err == nil {
		t.Fatal("Detect(99) should error (no algo)")
	}
}
