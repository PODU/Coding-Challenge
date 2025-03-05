// Package fractal implements the Mandelbrot escape-time iteration.
package fractal

import "math"

// EscapeRadius is the bailout radius. Anything past 2 is known to diverge, but
// a bigger radius gives smoother results from SmoothIters.
const (
	EscapeRadius   = 1 << 8
	EscapeRadiusSq = float64(EscapeRadius) * float64(EscapeRadius)
)

// Escape iterates z = z^2 + c for c = cr + ci*i, starting at z = 0, until |z|
// passes EscapeRadius or maxIter steps have run. It returns the step count,
// whether the point escaped, and the final |z|^2 (for smooth colouring).
func Escape(cr, ci float64, maxIter int) (iters int, escaped bool, magSq float64) {
	// The cardioid and the period-2 bulb are exactly in the set, and they
	// cover most of the interior, which would otherwise run all maxIter steps.
	if InMainCardioid(cr, ci) || InPeriod2Bulb(cr, ci) {
		return maxIter, false, 0
	}

	var zr, zi float64
	var zr2, zi2 float64
	for iters = 0; iters < maxIter; iters++ {
		zr2 = zr * zr
		zi2 = zi * zi
		if zr2+zi2 > EscapeRadiusSq {
			return iters, true, zr2 + zi2
		}
		zi = 2*zr*zi + ci
		zr = zr2 - zi2 + cr
	}
	return maxIter, false, zr*zr + zi*zi
}

// InMainCardioid reports whether c is inside the main cardioid:
// q*(q + (cr - 1/4)) <= ci^2/4, where q = (cr - 1/4)^2 + ci^2.
func InMainCardioid(cr, ci float64) bool {
	dx := cr - 0.25
	q := dx*dx + ci*ci
	return q*(q+dx) <= 0.25*ci*ci
}

// InPeriod2Bulb reports whether c is inside the disc of radius 1/4 at -1.
func InPeriod2Bulb(cr, ci float64) bool {
	dx := cr + 1.0
	return dx*dx+ci*ci <= 1.0/16.0
}

// SmoothIters returns a fractional iteration count, iters + 1 - log2(log|z|),
// for an escaped point. It avoids the banding of integer counts.
func SmoothIters(iters int, magSq float64) float64 {
	if magSq <= 1 {
		return float64(iters)
	}
	logZn := 0.5 * math.Log(magSq)
	nu := math.Log(logZn) / math.Ln2
	return float64(iters) + 1 - nu
}
