package deflate

import (
	"bytes"
	stdflate "compress/flate"
	"io"
	"math/rand"
	"testing"

	"gzipcc/internal/bitio"
)

// memSink is a simple in-memory deflate.Sink for tests.
type memSink struct{ buf []byte }

func (m *memSink) WriteByte(b byte) error { m.buf = append(m.buf, b); return nil }
func (m *memSink) CopyMatch(dist, length int) error {
	start := len(m.buf) - dist
	for i := 0; i < length; i++ {
		m.buf = append(m.buf, m.buf[start+i])
	}
	return nil
}
func (m *memSink) Len() int64 { return int64(len(m.buf)) }

func inflateBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	r := bitio.NewReader(bytes.NewReader(data))
	s := &memSink{}
	if err := Inflate(r, s); err != nil {
		t.Fatalf("Inflate: %v", err)
	}
	return s.buf
}

func compressBytes(t *testing.T, src []byte, level int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := Compress(&buf, src, level); err != nil {
		t.Fatalf("Compress: %v", err)
	}
	return buf.Bytes()
}

func compressForced(t *testing.T, src []byte, level, btype int) []byte {
	t.Helper()
	var buf bytes.Buffer
	bw := bitio.NewWriter(&buf)
	c := &compressor{src: src, bw: bw, level: level, cfg: levelConfigs[level], forceType: btype}
	c.run()
	bw.Align()
	if err := bw.Flush(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

var corpus = map[string][]byte{
	"empty":       {},
	"single":      {'A'},
	"allsame":     bytes.Repeat([]byte{'q'}, 5000),
	"text":        []byte("the quick brown fox jumps over the lazy dog. " + "the quick brown fox jumps over the lazy dog. "),
	"repeat":      bytes.Repeat([]byte("abcdefgh"), 4000),
	"abracadabra": []byte("abracadabra abracadabra abracadabra"),
}

func init() {
	rng := rand.New(rand.NewSource(1))
	rnd := make([]byte, 50000)
	rng.Read(rnd)
	corpus["random"] = rnd

	// Larger-than-window, multi-block input.
	big := make([]byte, 300000)
	for i := range big {
		big[i] = byte(i*7 + (i/97)*13)
	}
	corpus["bigpattern"] = big

	src := bytes.Repeat([]byte("func main() { fmt.Println(\"hello, world\") }\n"), 3000)
	corpus["source"] = src
}

func TestRoundTripAllLevels(t *testing.T) {
	for name, data := range corpus {
		for level := 0; level <= 9; level++ {
			comp := compressBytes(t, data, level)
			got := inflateBytes(t, comp)
			if !bytes.Equal(got, data) {
				t.Fatalf("%s level %d: round-trip mismatch (got %d bytes, want %d)", name, level, len(got), len(data))
			}
		}
	}
}

func TestForcedBlockTypes(t *testing.T) {
	for name, data := range corpus {
		if len(data) == 0 {
			continue
		}
		for btype := 0; btype <= 2; btype++ {
			comp := compressForced(t, data, 6, btype)
			got := inflateBytes(t, comp)
			if !bytes.Equal(got, data) {
				t.Fatalf("%s btype %d: forced round-trip mismatch", name, btype)
			}
			// Each forced path must also be decodable by the stdlib.
			r := stdflate.NewReader(bytes.NewReader(comp))
			out, err := io.ReadAll(r)
			if err != nil {
				t.Fatalf("%s btype %d: stdlib decode: %v", name, btype, err)
			}
			if !bytes.Equal(out, data) {
				t.Fatalf("%s btype %d: stdlib decode mismatch", name, btype)
			}
		}
	}
}

// TestInteropOurToStdlib: our DEFLATE output decodes with compress/flate.
func TestInteropOurToStdlib(t *testing.T) {
	for name, data := range corpus {
		for _, level := range []int{0, 1, 6, 9} {
			comp := compressBytes(t, data, level)
			r := stdflate.NewReader(bytes.NewReader(comp))
			got, err := io.ReadAll(r)
			if err != nil {
				t.Fatalf("%s level %d: stdlib flate decode: %v", name, level, err)
			}
			if !bytes.Equal(got, data) {
				t.Fatalf("%s level %d: stdlib decode mismatch", name, level)
			}
		}
	}
}

// TestInteropStdlibToOurs: compress/flate output decodes with our Inflate.
func TestInteropStdlibToOurs(t *testing.T) {
	for name, data := range corpus {
		for level := stdflate.NoCompression; level <= stdflate.BestCompression; level++ {
			var buf bytes.Buffer
			w, _ := stdflate.NewWriter(&buf, level)
			if _, err := w.Write(data); err != nil {
				t.Fatal(err)
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			got := inflateBytes(t, buf.Bytes())
			if !bytes.Equal(got, data) {
				t.Fatalf("%s stdlib level %d: our decode mismatch", name, level)
			}
		}
	}
}

func TestChooserSelectsStoredForRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	data := make([]byte, 20000)
	rng.Read(data)
	var buf bytes.Buffer
	bw := bitio.NewWriter(&buf)
	c := &compressor{src: data, bw: bw, level: 6, cfg: levelConfigs[6], forceType: -1}
	c.head = make([]int32, hashSize)
	for i := range c.head {
		c.head[i] = -1
	}
	c.prev = make([]int32, windowSize)
	// Manually drive one block and inspect the chosen type.
	c.deflate()
	bw.Align()
	bw.Flush()
	// First block's first byte low 3 bits: BFINAL + BTYPE.
	out := buf.Bytes()
	btype := (out[0] >> 1) & 3
	if btype != 0 {
		t.Errorf("random data: expected stored (0), got BTYPE %d", btype)
	}
}

func FuzzRoundTrip(f *testing.F) {
	for _, d := range corpus {
		f.Add(d, 6)
	}
	f.Fuzz(func(t *testing.T, data []byte, level int) {
		level = ((level % 10) + 10) % 10
		var buf bytes.Buffer
		if err := Compress(&buf, data, level); err != nil {
			t.Fatal(err)
		}
		got := inflateBytes(t, buf.Bytes())
		if !bytes.Equal(got, data) {
			t.Fatalf("round-trip mismatch: level %d, %d bytes", level, len(data))
		}
	})
}
