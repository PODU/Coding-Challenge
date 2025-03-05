package viewport

import (
	"math"
	"testing"
)

func TestFromBoundsCorners(t *testing.T) {
	v := FromBounds(-2, 2, -2, 2, 800, 800)

	re, im := v.ToPlane(0, 0)
	if re > -1.99 || im < 1.99 {
		t.Fatalf("pixel (0,0) => (%g,%g), want near (-2,+2)", re, im)
	}

	re, im = v.ToPlane(799, 799)
	if re < 1.99 || im > -1.99 {
		t.Fatalf("pixel (799,799) => (%g,%g), want near (+2,-2)", re, im)
	}

	// Center pixel maps near origin.
	re, im = v.ToPlane(400, 400)
	if math.Abs(re) > 0.01 || math.Abs(im) > 0.01 {
		t.Fatalf("center pixel => (%g,%g), want near (0,0)", re, im)
	}
}

func TestRoundTrip(t *testing.T) {
	v := FromBounds(-2, 1, -1.5, 1.5, 640, 480)
	for _, px := range []int{0, 1, 100, 320, 639} {
		for _, py := range []int{0, 50, 240, 479} {
			re, im := v.ToPlane(px, py)
			gx, gy := v.ToPixel(re, im)
			if gx != px || gy != py {
				t.Fatalf("round-trip (%d,%d) -> (%g,%g) -> (%d,%d)", px, py, re, im, gx, gy)
			}
		}
	}
}

func TestFromCenterReproducesDefault(t *testing.T) {
	v := FromCenter(0, 0, 1, 800, 800)
	const eps = 1e-9
	if math.Abs(v.MinRe-(-2)) > eps || math.Abs(v.MaxRe-2) > eps ||
		math.Abs(v.MinIm-(-2)) > eps || math.Abs(v.MaxIm-2) > eps {
		t.Fatalf("FromCenter(0,0,1) => %+v, want -2..2 bounds", v)
	}
}

func TestSquarePixelInvariant(t *testing.T) {
	// Center mode must keep pixels square even for non-square images.
	v := FromCenter(-0.743, 0.131, 200, 1920, 1080)
	dRe, dIm := v.PixelSize()
	if math.Abs(dRe-dIm) > 1e-12 {
		t.Fatalf("pixels not square: dRe=%g dIm=%g", dRe, dIm)
	}
	if v.AspectMismatch() {
		t.Fatalf("center-mode viewport reported aspect mismatch")
	}
}

func TestAspectMismatchDetected(t *testing.T) {
	// Square bounds into a non-square image stretches.
	v := FromBounds(-2, 2, -2, 2, 1920, 1080)
	if !v.AspectMismatch() {
		t.Fatalf("expected aspect mismatch for square bounds in 1920x1080")
	}
}
