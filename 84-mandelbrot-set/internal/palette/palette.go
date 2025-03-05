// Package palette maps escape-time results to colours.
package palette

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"mandelbrot/internal/fractal"
)

// Palette converts one escape-time result into an RGBA colour.
type Palette interface {
	Color(iters int, escaped bool, magSq float64, maxIter int) color.RGBA
}

// base colours escaped points by ramp(t), t being the iteration count scaled
// to [0,1].
type base struct {
	inside color.RGBA
	smooth bool
	ramp   func(t float64) color.RGBA
}

func (b base) Color(iters int, escaped bool, magSq float64, maxIter int) color.RGBA {
	if !escaped {
		return b.inside
	}
	n := float64(iters)
	if b.smooth {
		n = fractal.SmoothIters(iters, magSq)
	}
	t := math.Min(math.Max(n/float64(maxIter), 0), 1)
	return b.ramp(t)
}

// New returns the named palette. inside is the colour for points in the set.
func New(name string, inside color.RGBA, smooth bool) (Palette, error) {
	ramp, ok := ramps[strings.ToLower(name)]
	if !ok {
		return nil, fmt.Errorf("unknown palette %q (choose grayscale|fire|blue|rainbow)", name)
	}
	return base{inside: inside, smooth: smooth, ramp: ramp}, nil
}

// Names returns the supported palette names (for help text / validation).
func Names() []string { return []string{"grayscale", "fire", "blue", "rainbow"} }

var ramps = map[string]func(float64) color.RGBA{
	"grayscale": grayscaleRamp,
	"fire":      fireRamp,
	"blue":      blueRamp,
	"rainbow":   rainbowRamp,
}

func grayscaleRamp(t float64) color.RGBA {
	v := uint8(math.Round(t * 255))
	return color.RGBA{v, v, v, 255}
}

// fireRamp: black -> red -> orange -> yellow -> white.
func fireRamp(t float64) color.RGBA {
	r := clamp8(t * 3)
	g := clamp8(t*3 - 1)
	b := clamp8(t*3 - 2)
	return color.RGBA{r, g, b, 255}
}

// blueRamp: dark navy -> bright cyan-white.
func blueRamp(t float64) color.RGBA {
	r := clamp8(t * t)
	g := clamp8(math.Sqrt(t))
	b := clamp8(0.3 + 0.7*t)
	return color.RGBA{r, g, b, 255}
}

func rainbowRamp(t float64) color.RGBA {
	return hsv(t*360, 1, 1)
}

func clamp8(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return uint8(math.Round(v * 255))
}

// hsv takes h in degrees and s, v in [0,1].
func hsv(h, s, v float64) color.RGBA {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := v - c
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return color.RGBA{
		R: uint8(math.Round((r + m) * 255)),
		G: uint8(math.Round((g + m) * 255)),
		B: uint8(math.Round((b + m) * 255)),
		A: 255,
	}
}

// ParseHexColor parses "#RRGGBB".
func ParseHexColor(s string) (color.RGBA, error) {
	s = strings.TrimSpace(s)
	if len(s) != 7 || s[0] != '#' {
		return color.RGBA{}, fmt.Errorf("invalid colour %q: want #RRGGBB", s)
	}
	var r, g, b uint8
	n, err := fmt.Sscanf(s, "#%02x%02x%02x", &r, &g, &b)
	if err != nil || n != 3 {
		return color.RGBA{}, fmt.Errorf("invalid colour %q: want #RRGGBB", s)
	}
	return color.RGBA{R: r, G: g, B: b, A: 255}, nil
}
