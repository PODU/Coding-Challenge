package sha256

import (
	stdsha "crypto/sha256"
	"encoding/hex"
	"math/rand"
	"testing"
)

func TestKnownVectors(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"abc", "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
	}
	for _, c := range cases {
		sum := Sum([]byte(c.in))
		got := hex.EncodeToString(sum[:])
		if got != c.want {
			t.Errorf("SHA256(%q) = %s want %s", c.in, got, c.want)
		}
	}
}

func TestDifferential(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	for n := 0; n <= 300; n++ {
		data := make([]byte, n)
		rng.Read(data)
		if Sum(data) != stdsha.Sum256(data) {
			t.Fatalf("len=%d mismatch", n)
		}
	}
}

func TestStreamingInvariance(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	data := make([]byte, 4096)
	rng.Read(data)
	oneShot := Sum(data)
	for _, chunk := range []int{1, 7, 64, 65} {
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
			t.Fatalf("chunk=%d streaming mismatch", chunk)
		}
	}
}

func FuzzSHA256(f *testing.F) {
	f.Add([]byte("abc"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if Sum(data) != stdsha.Sum256(data) {
			t.Errorf("mismatch on %d bytes", len(data))
		}
	})
}
