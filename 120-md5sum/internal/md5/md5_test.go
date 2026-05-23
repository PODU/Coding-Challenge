package md5

import (
	stdmd5 "crypto/md5"
	"encoding/hex"
	"math/rand"
	"strings"
	"testing"
)

// TestKnownVectors checks the RFC 1321 / appendix A.5 known-answer vectors.
func TestKnownVectors(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "d41d8cd98f00b204e9800998ecf8427e"},
		{"a", "0cc175b9c0f1b6a831c399e269772661"},
		{"abc", "900150983cd24fb0d6963f7d28e17f72"},
		{"message digest", "f96b697d7cb7938d525a2f31aaf161d0"},
		{"abcdefghijklmnopqrstuvwxyz", "c3fcd3d76192e4007dfb496cca67e13b"},
		{"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789", "d174ab98d277d9f5a5611c2c9f419d9f"},
		{strings.Repeat("1234567890", 8), "57edf4a22be3c955ac49da2e2107b67a"},
	}
	for _, c := range cases {
		got := hex.EncodeToString(sum(c.in))
		if got != c.want {
			t.Errorf("MD5(%q) = %s, want %s", c.in, got, c.want)
		}
	}
}

func sum(s string) []byte {
	d := New()
	d.Write([]byte(s))
	return d.Sum(nil)
}

// TestDifferential compares our digest with crypto/md5 across every length in
// 0..400 (covering all padding boundaries: 55/56/57, 63/64/65, 119/120, ...).
func TestDifferential(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for n := 0; n <= 400; n++ {
		data := make([]byte, n)
		rng.Read(data)
		got := Sum(data)
		want := stdmd5.Sum(data)
		if got != want {
			t.Fatalf("len=%d: got %x want %x", n, got, want)
		}
	}
}

// TestDifferentialLarge cross-checks a few multi-megabyte random inputs.
func TestDifferentialLarge(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	for _, n := range []int{1 << 16, 1<<20 + 7, 3<<20 + 123} {
		data := make([]byte, n)
		rng.Read(data)
		if Sum(data) != stdmd5.Sum(data) {
			t.Fatalf("large len=%d mismatch", n)
		}
	}
}

// TestStreamingInvariance asserts that splitting the input into many small
// Writes yields the same digest as one big Write.
func TestStreamingInvariance(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	data := make([]byte, 5000)
	rng.Read(data)

	oneShot := Sum(data)

	for _, chunk := range []int{1, 7, 63, 64, 65, 100} {
		d := New()
		for i := 0; i < len(data); i += chunk {
			end := i + chunk
			if end > len(data) {
				end = len(data)
			}
			d.Write(data[i:end])
		}
		var got [Size]byte
		copy(got[:], d.Sum(nil))
		if got != oneShot {
			t.Errorf("chunk=%d: streaming digest %x != one-shot %x", chunk, got, oneShot)
		}
	}
}

// TestSumDoesNotMutate confirms Sum can be called repeatedly and writing can
// continue afterwards (Sum operates on a copy of the state).
func TestSumDoesNotMutate(t *testing.T) {
	d := New()
	d.Write([]byte("abc"))
	first := hex.EncodeToString(d.Sum(nil))
	second := hex.EncodeToString(d.Sum(nil))
	if first != second {
		t.Fatalf("Sum mutated state: %s != %s", first, second)
	}
	d.Write([]byte("def"))
	got := hex.EncodeToString(d.Sum(nil))
	want := hex.EncodeToString(func() []byte { s := stdmd5.Sum([]byte("abcdef")); return s[:] }())
	if got != want {
		t.Fatalf("continued write: got %s want %s", got, want)
	}
}

// FuzzMD5 fuzzes our implementation against crypto/md5.
func FuzzMD5(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("abc"))
	f.Add([]byte(strings.Repeat("x", 130)))
	f.Fuzz(func(t *testing.T, data []byte) {
		if Sum(data) != stdmd5.Sum(data) {
			t.Errorf("mismatch on %d bytes", len(data))
		}
	})
}
