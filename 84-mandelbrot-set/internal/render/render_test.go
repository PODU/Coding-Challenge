package render

import (
	"bytes"
	"image/color"
	"runtime"
	"testing"

	"mandelbrot/internal/palette"
	"mandelbrot/internal/viewport"
)

func defaultConfig(w, h, parallel int) Config {
	pal, _ := palette.New("rainbow", color.RGBA{A: 255}, false)
	return Config{
		View:     viewport.FromBounds(-2, 2, -2, 2, w, h),
		Palette:  pal,
		MaxIter:  500,
		Parallel: parallel,
	}
}

func TestRenderDimensions(t *testing.T) {
	img := Render(defaultConfig(64, 48, 0))
	b := img.Bounds()
	if b.Dx() != 64 || b.Dy() != 48 {
		t.Fatalf("image bounds = %v, want 64x48", b)
	}
}

func TestCenterPixelInSet(t *testing.T) {
	// Origin is in the set => inside colour (black here).
	inside := color.RGBA{A: 255}
	pal, _ := palette.New("rainbow", inside, false)
	cfg := Config{
		View:    viewport.FromBounds(-2, 2, -2, 2, 65, 65),
		Palette: pal,
		MaxIter: 500,
	}
	img := Render(cfg)
	got := img.RGBAAt(32, 32)
	if got != inside {
		t.Fatalf("center pixel = %v, want in-set colour %v", got, inside)
	}
}

func TestCornerPixelEscapes(t *testing.T) {
	inside := color.RGBA{A: 255}
	pal, _ := palette.New("rainbow", inside, false)
	cfg := Config{
		View:    viewport.FromBounds(-2, 2, -2, 2, 64, 64),
		Palette: pal,
		MaxIter: 500,
	}
	img := Render(cfg)
	if got := img.RGBAAt(0, 0); got == inside {
		t.Fatalf("corner pixel = in-set colour %v, want escaped colour", got)
	}
}

func TestDeterminism(t *testing.T) {
	a := Render(defaultConfig(96, 96, 0))
	b := Render(defaultConfig(96, 96, 0))
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Fatalf("identical configs produced different pixels")
	}
}

func TestParallelEqualsSerial(t *testing.T) {
	serial := Render(defaultConfig(128, 96, 1))
	par := Render(defaultConfig(128, 96, runtime.NumCPU()*2))
	if !bytes.Equal(serial.Pix, par.Pix) {
		t.Fatalf("parallel output differs from serial output")
	}
}

func TestSmoothRenderRuns(t *testing.T) {
	pal, _ := palette.New("fire", color.RGBA{A: 255}, true)
	cfg := Config{
		View:    viewport.FromCenter(-0.743, 0.131, 50, 80, 80),
		Palette: pal,
		MaxIter: 800,
	}
	img := Render(cfg)
	if img.Bounds().Dx() != 80 {
		t.Fatalf("unexpected dimensions")
	}
}
