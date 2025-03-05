package fractal

import (
	"math"
	"testing"
)

func TestEscapeKnownPoints(t *testing.T) {
	const maxIter = 1000
	tests := []struct {
		name        string
		cr, ci      float64
		wantEscaped bool
	}{
		{"origin in set (c=0)", 0, 0, false},
		{"c=-1 period-2 in set", -1, 0, false},
		{"c=2 escapes", 2, 0, true},
		{"c=1 escapes", 1, 0, true},
		{"c=0.3 escapes", 0.3, 0, true},
		{"c=0.4+0.4i escapes off-axis", 0.4, 0.4, true},
		{"near origin inside cardioid", -0.1, 0.1, false},
		// c=-0.5+0.6i looks like it should escape, but it's actually bounded
		// (in the set).
		{"c=-0.5+0.6i is in set", -0.5, 0.6, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iters, escaped, _ := Escape(tt.cr, tt.ci, maxIter)
			if escaped != tt.wantEscaped {
				t.Fatalf("Escape(%g,%g) escaped=%v, want %v (iters=%d)",
					tt.cr, tt.ci, escaped, tt.wantEscaped, iters)
			}
			if !escaped && iters != maxIter {
				t.Fatalf("Escape(%g,%g) in-set but iters=%d, want %d",
					tt.cr, tt.ci, iters, maxIter)
			}
			if escaped && iters >= maxIter {
				t.Fatalf("Escape(%g,%g) escaped but iters=%d >= maxIter", tt.cr, tt.ci, iters)
			}
		})
	}
}

func TestEscapeC2Immediate(t *testing.T) {
	// c=2: z goes 0, 2, 6, 38, ... escapes very quickly.
	iters, escaped, _ := Escape(2, 0, 1000)
	if !escaped {
		t.Fatalf("c=2 should escape")
	}
	if iters > 5 {
		t.Fatalf("c=2 should escape almost immediately, got iters=%d", iters)
	}
}

func TestEscapeMonotonicity(t *testing.T) {
	// Increasing maxIter must never reclassify an escaped point as in-set.
	pts := [][2]float64{{2, 0}, {1, 0}, {0.3, 0}, {-0.5, 0.6}, {0.35, 0.35}}
	for _, p := range pts {
		_, esc100, _ := Escape(p[0], p[1], 100)
		_, esc5000, _ := Escape(p[0], p[1], 5000)
		if esc100 && !esc5000 {
			t.Fatalf("point (%g,%g) escaped at maxIter=100 but not at 5000", p[0], p[1])
		}
	}
}

func TestCardioidEarlyOutMatchesBruteForce(t *testing.T) {
	// For a grid of points, the cardioid/bulb early-out must agree with a
	// brute-force loop (a point flagged interior must never escape).
	const maxIter = 2000
	for px := -250; px <= 100; px++ {
		for py := 0; py <= 125; py++ {
			cr := float64(px) / 100
			ci := float64(py) / 100
			if InMainCardioid(cr, ci) || InPeriod2Bulb(cr, ci) {
				if escaped := bruteEscaped(cr, ci, maxIter); escaped {
					t.Fatalf("interior test flagged (%g,%g) but brute force escaped", cr, ci)
				}
			}
		}
	}
}

// bruteEscaped runs the raw loop with no early-out.
func bruteEscaped(cr, ci float64, maxIter int) bool {
	var zr, zi float64
	for i := 0; i < maxIter; i++ {
		zr2 := zr * zr
		zi2 := zi * zi
		if zr2+zi2 > EscapeRadiusSq {
			return true
		}
		zi = 2*zr*zi + ci
		zr = zr2 - zi2 + cr
	}
	return false
}

func TestSmoothItersFinite(t *testing.T) {
	iters, escaped, magSq := Escape(0.35, 0.35, 1000)
	if !escaped {
		t.Skip("test point did not escape")
	}
	mu := SmoothIters(iters, magSq)
	if math.IsNaN(mu) || math.IsInf(mu, 0) {
		t.Fatalf("SmoothIters returned non-finite %v", mu)
	}
	if mu < 0 {
		t.Fatalf("SmoothIters returned negative %v", mu)
	}
}
