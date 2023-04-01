package huffman

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestLesMiserables exercises the challenge's canonical corpus when present.
// The file is not committed (see .gitignore); fetch it from Project Gutenberg
// and drop it at testdata/les_miserables.txt to enable this test. When the
// file is absent the test is skipped so the suite stays green offline.
func TestLesMiserables(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "les_miserables.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("corpus not present (%v); skipping", err)
	}

	var freq FreqTable
	for _, b := range data {
		freq[b]++
	}
	// Validation frequencies cited by the challenge blog.
	if freq['X'] != 333 {
		t.Errorf("freq['X'] = %d, want 333", freq['X'])
	}
	if freq['t'] != 223000 {
		t.Errorf("freq['t'] = %d, want 223000", freq['t'])
	}

	var compressed bytes.Buffer
	stats, err := CompressBytes(data, &compressed)
	if err != nil {
		t.Fatal(err)
	}
	if stats.OutputBytes >= stats.InputBytes {
		t.Fatalf("output %d not smaller than input %d", stats.OutputBytes, stats.InputBytes)
	}
	t.Logf("Les Mis: %d -> %d bytes (%.1f%%, ratio %.2fx)",
		stats.InputBytes, stats.OutputBytes, stats.Percent(), stats.Ratio())

	var out bytes.Buffer
	if err := Decompress(bytes.NewReader(compressed.Bytes()), &out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), data) {
		t.Fatal("Les Mis round-trip mismatch")
	}
}
