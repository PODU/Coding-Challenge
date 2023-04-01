package huffman

import (
	"bytes"
	"math/rand"
	"testing"
	"testing/quick"
)

// roundTrip compresses then decompresses data and returns the recovered bytes.
func roundTrip(t *testing.T, data []byte) []byte {
	t.Helper()
	var compressed bytes.Buffer
	if _, err := CompressBytes(data, &compressed); err != nil {
		t.Fatalf("compress: %v", err)
	}
	var out bytes.Buffer
	if err := Decompress(bytes.NewReader(compressed.Bytes()), &out); err != nil {
		t.Fatalf("decompress: %v", err)
	}
	return out.Bytes()
}

func TestRoundTripFixtures(t *testing.T) {
	all256 := make([]byte, 256)
	for i := range all256 {
		all256[i] = byte(i)
	}
	cases := map[string][]byte{
		"empty":           {},
		"single byte":     {'A'},
		"repeated symbol": bytes.Repeat([]byte{'A'}, 1000),
		"two symbols":     bytes.Repeat([]byte("AB"), 500),
		"all 256 bytes":   all256,
		"all 256 x100":    bytes.Repeat(all256, 100),
		"ascii":           []byte("the quick brown fox jumps over the lazy dog"),
		"newlines/tabs":   []byte("a\tb\nc\r\nd"),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			got := roundTrip(t, data)
			if !bytes.Equal(got, data) {
				t.Fatalf("round-trip mismatch: got %d bytes, want %d", len(got), len(data))
			}
		})
	}
}

func TestRoundTripBinary(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	data := make([]byte, 50000)
	rng.Read(data)
	got := roundTrip(t, data)
	if !bytes.Equal(got, data) {
		t.Fatal("binary round-trip mismatch")
	}
}

// TestRoundTripProperty is the strongest correctness signal: thousands of
// random byte slices of random length must survive compress->decompress.
func TestRoundTripProperty(t *testing.T) {
	f := func(data []byte) bool {
		return bytes.Equal(roundTrip(t, data), data)
	}
	if err := quick.Check(f, &quick.Config{MaxCount: 2000}); err != nil {
		t.Fatal(err)
	}
}

func TestEmptyFileProducesHeaderOnly(t *testing.T) {
	var compressed bytes.Buffer
	stats, err := CompressBytes(nil, &compressed)
	if err != nil {
		t.Fatal(err)
	}
	// MAGIC(4) + SYMBOL_COUNT(8) + TABLE_LEN(2) = 14, no entries, no payload.
	if compressed.Len() != 14 {
		t.Fatalf("empty file header size = %d, want 14", compressed.Len())
	}
	if stats.InputBytes != 0 || stats.OutputBytes != 14 {
		t.Fatalf("unexpected stats %+v", stats)
	}
}

func TestSingleSymbolCode(t *testing.T) {
	var freq FreqTable
	freq['Z'] = 7
	codes := buildCodes(buildTree(&freq))
	if codes.length['Z'] != 1 {
		t.Fatalf("single-symbol code length = %d, want 1", codes.length['Z'])
	}
}

func TestFrequencyCounting(t *testing.T) {
	data := []byte("aaabbc")
	var freq FreqTable
	tbl, total, err := CountFrequencies(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	freq = tbl
	if total != 6 {
		t.Fatalf("total = %d, want 6", total)
	}
	if freq['a'] != 3 || freq['b'] != 2 || freq['c'] != 1 {
		t.Fatalf("bad counts: a=%d b=%d c=%d", freq['a'], freq['b'], freq['c'])
	}
	if freq.DistinctCount() != 3 {
		t.Fatalf("distinct = %d, want 3", freq.DistinctCount())
	}
}

// TestPrefixFree verifies no generated code is a prefix of another by building
// a trie from the codes and ensuring every leaf is exclusively a code endpoint.
func TestPrefixFree(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	var freq FreqTable
	for i := 0; i < 256; i++ {
		freq[i] = uint64(rng.Intn(1000) + 1)
	}
	codes := buildCodes(buildTree(&freq))

	type trieNode struct {
		child [2]*trieNode
		isEnd bool
	}
	root := &trieNode{}
	for sym := 0; sym < 256; sym++ {
		n := codes.length[sym]
		if n == 0 {
			continue
		}
		cur := root
		for i := int(n) - 1; i >= 0; i-- {
			bit := (codes.bits[sym] >> i) & 1
			if cur.isEnd {
				t.Fatalf("code for %d has a proper prefix that is also a code", sym)
			}
			if cur.child[bit] == nil {
				cur.child[bit] = &trieNode{}
			}
			cur = cur.child[bit]
		}
		if cur.isEnd || cur.child[0] != nil || cur.child[1] != nil {
			t.Fatalf("code for %d collides with another code", sym)
		}
		cur.isEnd = true
	}
}

func TestDeterministicTree(t *testing.T) {
	data := []byte("the quick brown fox jumps over the lazy dog, again and again")
	var a, b bytes.Buffer
	if _, err := CompressBytes(data, &a); err != nil {
		t.Fatal(err)
	}
	if _, err := CompressBytes(data, &b); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("compression is not deterministic")
	}
}

func TestBitWriterReaderRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	type chunk struct {
		v uint64
		n uint8
	}
	var chunks []chunk
	var buf bytes.Buffer
	bw := NewBitWriter(&buf)
	for i := 0; i < 5000; i++ {
		n := uint8(rng.Intn(40) + 1) // 1..40 bits
		v := rng.Uint64() & ((1 << n) - 1)
		chunks = append(chunks, chunk{v, n})
		if err := bw.WriteBits(v, n); err != nil {
			t.Fatal(err)
		}
	}
	if err := bw.Close(); err != nil {
		t.Fatal(err)
	}
	br := NewBitReader(&buf)
	for ci, c := range chunks {
		var got uint64
		for i := uint8(0); i < c.n; i++ {
			bit, err := br.ReadBit()
			if err != nil {
				t.Fatalf("chunk %d bit %d: %v", ci, i, err)
			}
			got = (got << 1) | uint64(bit)
		}
		if got != c.v {
			t.Fatalf("chunk %d: got %b want %b (n=%d)", ci, got, c.v, c.n)
		}
	}
}

func TestHeaderRoundTrip(t *testing.T) {
	for _, distinct := range []int{0, 1, 2, 256} {
		var freq FreqTable
		var total uint64
		for i := 0; i < distinct; i++ {
			freq[i] = uint64(i + 1)
			total += uint64(i + 1)
		}
		var buf bytes.Buffer
		if err := writeHeader(&buf, total, &freq); err != nil {
			t.Fatalf("distinct=%d write: %v", distinct, err)
		}
		h, err := readHeader(bytes.NewReader(buf.Bytes()))
		if err != nil {
			t.Fatalf("distinct=%d read: %v", distinct, err)
		}
		if h.symbolCount != total {
			t.Fatalf("distinct=%d symbolCount=%d want %d", distinct, h.symbolCount, total)
		}
		if h.freq != freq {
			t.Fatalf("distinct=%d freq table mismatch", distinct)
		}
	}
}

func TestRejectBadMagic(t *testing.T) {
	var out bytes.Buffer
	err := Decompress(bytes.NewReader([]byte("NOPEnotahuffmanfile")), &out)
	if err != ErrBadMagic {
		t.Fatalf("got %v, want ErrBadMagic", err)
	}
}

func TestRejectTruncatedHeader(t *testing.T) {
	var compressed bytes.Buffer
	if _, err := CompressBytes([]byte("hello world hello world"), &compressed); err != nil {
		t.Fatal(err)
	}
	truncated := compressed.Bytes()[:10] // cut inside the header
	var out bytes.Buffer
	if err := Decompress(bytes.NewReader(truncated), &out); err != ErrCorrupt {
		t.Fatalf("got %v, want ErrCorrupt", err)
	}
}

func TestRejectTruncatedPayload(t *testing.T) {
	var compressed bytes.Buffer
	if _, err := CompressBytes([]byte("hello world hello world foo bar baz"), &compressed); err != nil {
		t.Fatal(err)
	}
	b := compressed.Bytes()
	truncated := b[:len(b)-1] // drop a payload byte
	var out bytes.Buffer
	if err := Decompress(bytes.NewReader(truncated), &out); err != ErrCorrupt {
		t.Fatalf("got %v, want ErrCorrupt", err)
	}
}

func TestCompressionShrinksText(t *testing.T) {
	// Repetitive English-like prose should compress well below original size.
	text := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog. "), 2000)
	var compressed bytes.Buffer
	stats, err := CompressBytes(text, &compressed)
	if err != nil {
		t.Fatal(err)
	}
	if stats.OutputBytes >= stats.InputBytes {
		t.Fatalf("output %d not smaller than input %d", stats.OutputBytes, stats.InputBytes)
	}
	if r := stats.Ratio(); r < 1.5 {
		t.Fatalf("ratio %.2f lower than expected for English text", r)
	}
}
