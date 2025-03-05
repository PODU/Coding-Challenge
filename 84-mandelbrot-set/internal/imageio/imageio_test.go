package imageio

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func sample(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}
	return img
}

func TestEncodeDecodeDimensions(t *testing.T) {
	const w, h = 40, 24
	var buf bytes.Buffer
	if err := Encode(&buf, sample(w, h), 72); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("decode failed (invalid PNG): %v", err)
	}
	b := img.Bounds()
	if b.Dx() != w || b.Dy() != h {
		t.Fatalf("decoded dims = %v, want %dx%d", b, w, h)
	}
}

func TestValidPNGMagic(t *testing.T) {
	var buf bytes.Buffer
	if err := Encode(&buf, sample(8, 8), 0); err != nil {
		t.Fatal(err)
	}
	magic := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}
	if !bytes.HasPrefix(buf.Bytes(), magic) {
		t.Fatalf("output missing PNG magic bytes")
	}
}

func TestPHYsRoundTrip(t *testing.T) {
	const dpi = 300
	var buf bytes.Buffer
	if err := Encode(&buf, sample(16, 16), dpi); err != nil {
		t.Fatal(err)
	}
	ppm, ok := ReadPHYs(buf.Bytes())
	if !ok {
		t.Fatalf("no pHYs chunk found")
	}
	want := DPIToPPM(dpi)
	if ppm != want {
		t.Fatalf("pHYs ppm = %d, want %d", ppm, want)
	}
	// Sanity: chunk position must not corrupt the image.
	if _, err := png.Decode(bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatalf("PNG with pHYs no longer decodes: %v", err)
	}
}

func TestNoPHYsWhenDPIZero(t *testing.T) {
	var buf bytes.Buffer
	if err := Encode(&buf, sample(8, 8), 0); err != nil {
		t.Fatal(err)
	}
	if _, ok := ReadPHYs(buf.Bytes()); ok {
		t.Fatalf("did not expect pHYs chunk when dpi=0")
	}
}

func TestDPIToPPM(t *testing.T) {
	if got := DPIToPPM(72); got != 2835 {
		t.Fatalf("DPIToPPM(72) = %d, want 2835", got)
	}
}
