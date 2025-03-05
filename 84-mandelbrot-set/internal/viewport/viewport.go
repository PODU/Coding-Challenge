// Package viewport maps between pixel coordinates and the complex plane.
package viewport

import "math"

// DefaultSpan is the width of the full-set view (-2..2). Magnification is
// relative to it.
const DefaultSpan = 4.0

// Viewport is a rectangle of the complex plane and the image size it maps to.
type Viewport struct {
	MinRe, MaxRe float64
	MinIm, MaxIm float64
	Width        int
	Height       int
}

// FromBounds builds a Viewport from a bounding box. The aspect ratio is not
// adjusted; see AspectMismatch.
func FromBounds(minRe, maxRe, minIm, maxIm float64, width, height int) Viewport {
	return Viewport{
		MinRe:  minRe,
		MaxRe:  maxRe,
		MinIm:  minIm,
		MaxIm:  maxIm,
		Width:  width,
		Height: height,
	}
}

// FromCenter builds a Viewport around a centre point. Magnification 1 spans
// DefaultSpan on the real axis; the imaginary span follows from the image
// shape so pixels stay square.
func FromCenter(centerRe, centerIm, magnification float64, width, height int) Viewport {
	halfRe := (DefaultSpan / 2) / magnification
	halfIm := halfRe * float64(height) / float64(width)
	return Viewport{
		MinRe:  centerRe - halfRe,
		MaxRe:  centerRe + halfRe,
		MinIm:  centerIm - halfIm,
		MaxIm:  centerIm + halfIm,
		Width:  width,
		Height: height,
	}
}

// ToPlane returns the point at the centre of pixel (px, py). Imaginary values
// grow upwards.
func (v Viewport) ToPlane(px, py int) (re, im float64) {
	re = v.MinRe + (float64(px)+0.5)/float64(v.Width)*(v.MaxRe-v.MinRe)
	im = v.MaxIm - (float64(py)+0.5)/float64(v.Height)*(v.MaxIm-v.MinIm)
	return re, im
}

// ToPixel is the inverse of ToPlane, to the nearest pixel.
func (v Viewport) ToPixel(re, im float64) (px, py int) {
	fx := (re-v.MinRe)/(v.MaxRe-v.MinRe)*float64(v.Width) - 0.5
	fy := (v.MaxIm-im)/(v.MaxIm-v.MinIm)*float64(v.Height) - 0.5
	return int(math.Round(fx)), int(math.Round(fy))
}

// PixelSize returns the real- and imaginary-axis size of a single pixel.
func (v Viewport) PixelSize() (dRe, dIm float64) {
	return (v.MaxRe - v.MinRe) / float64(v.Width),
		(v.MaxIm - v.MinIm) / float64(v.Height)
}

// AspectMismatch reports whether pixels would come out non-square, i.e. the
// image will look stretched.
func (v Viewport) AspectMismatch() bool {
	dRe, dIm := v.PixelSize()
	if dRe == 0 || dIm == 0 {
		return false
	}
	return math.Abs(dRe-dIm) > 1e-9*math.Max(math.Abs(dRe), math.Abs(dIm))
}
