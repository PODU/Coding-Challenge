package huffman

import (
	"bytes"
	"math/rand"
	"testing"
)

// englishish builds a pseudo-English byte slice with a skewed, text-like
// frequency distribution so the benchmark exercises a realistic tree.
func englishish(n int) []byte {
	const alphabet = "etaoinshrdlcumwfgypbvkjxqz ETAOINSHRDLCU.,\n"
	rng := rand.New(rand.NewSource(99))
	out := make([]byte, n)
	for i := range out {
		// Bias toward the front of the alphabet (more frequent letters).
		idx := int(float64(len(alphabet)) * rng.Float64() * rng.Float64())
		if idx >= len(alphabet) {
			idx = len(alphabet) - 1
		}
		out[i] = alphabet[idx]
	}
	return out
}

func BenchmarkCompress(b *testing.B) {
	data := englishish(3_200_000) // ~Les Mis scale
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		if _, err := CompressBytes(data, &buf); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecompress(b *testing.B) {
	data := englishish(3_200_000)
	var compressed bytes.Buffer
	if _, err := CompressBytes(data, &compressed); err != nil {
		b.Fatal(err)
	}
	payload := compressed.Bytes()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out bytes.Buffer
		if err := Decompress(bytes.NewReader(payload), &out); err != nil {
			b.Fatal(err)
		}
	}
}
