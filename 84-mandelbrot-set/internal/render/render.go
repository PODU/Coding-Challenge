// Package render draws a viewport into an image, one scanline per job.
package render

import (
	"image"
	"runtime"
	"sync"
	"sync/atomic"

	"mandelbrot/internal/fractal"
	"mandelbrot/internal/palette"
	"mandelbrot/internal/viewport"
)

// Config describes a single render.
type Config struct {
	View     viewport.Viewport
	Palette  palette.Palette
	MaxIter  int
	Parallel int // worker goroutines; <= 0 means runtime.NumCPU().
}

func (c Config) workers() int {
	n := c.Parallel
	if n <= 0 {
		n = runtime.NumCPU()
	}
	if n > c.View.Height {
		n = c.View.Height
	}
	if n < 1 {
		n = 1
	}
	return n
}

// Render draws the image. Each pixel depends only on its coordinates, so the
// result is the same whatever the worker count.
func Render(c Config) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, c.View.Width, c.View.Height))
	n := c.workers()

	if n == 1 {
		for py := 0; py < c.View.Height; py++ {
			c.renderRow(img, py)
		}
		return img
	}

	// Workers pull row numbers from a shared counter. Rows don't overlap in
	// Pix, so the writes need no locking.
	var next atomic.Int64
	var wg sync.WaitGroup
	wg.Add(n)
	for w := 0; w < n; w++ {
		go func() {
			defer wg.Done()
			for {
				py := int(next.Add(1) - 1)
				if py >= c.View.Height {
					return
				}
				c.renderRow(img, py)
			}
		}()
	}
	wg.Wait()
	return img
}

func (c Config) renderRow(img *image.RGBA, py int) {
	row := img.PixOffset(0, py)
	for px := 0; px < c.View.Width; px++ {
		re, im := c.View.ToPlane(px, py)
		iters, escaped, magSq := fractal.Escape(re, im, c.MaxIter)
		col := c.Palette.Color(iters, escaped, magSq, c.MaxIter)
		off := row + px*4
		img.Pix[off+0] = col.R
		img.Pix[off+1] = col.G
		img.Pix[off+2] = col.B
		img.Pix[off+3] = col.A
	}
}
