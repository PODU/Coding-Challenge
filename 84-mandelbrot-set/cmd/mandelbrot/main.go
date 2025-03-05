// Command mandelbrot renders the Mandelbrot set to a PNG.
//
// The region is either a bounding box (-minRe etc.) or a centre point plus a
// magnification. With no flags it draws the full set, -2..2 on both axes.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"mandelbrot/internal/imageio"
	"mandelbrot/internal/palette"
	"mandelbrot/internal/render"
	"mandelbrot/internal/viewport"
)

// maxPixels caps the image buffer at about 1.6 GiB.
const maxPixels = 400_000_000

type options struct {
	width, height int
	out           string
	dpi           int

	minRe, maxRe float64
	minIm, maxIm float64

	centerRe, centerIm float64
	mag                float64

	iter     int
	parallel int

	paletteName string
	smooth      bool
	inside      string
}

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "mandelbrot:", err)
		os.Exit(1)
	}
}

func run(args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("mandelbrot", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var o options
	fs.IntVar(&o.width, "width", 800, "output width in pixels")
	fs.IntVar(&o.height, "height", 800, "output height in pixels")
	fs.StringVar(&o.out, "out", "mandelbrot.png", "output PNG path")
	fs.IntVar(&o.dpi, "dpi", 72, "DPI metadata for print")

	fs.Float64Var(&o.minRe, "minRe", -2.0, "min real (bounds mode)")
	fs.Float64Var(&o.maxRe, "maxRe", 2.0, "max real (bounds mode)")
	fs.Float64Var(&o.minIm, "minIm", -2.0, "min imaginary (bounds mode)")
	fs.Float64Var(&o.maxIm, "maxIm", 2.0, "max imaginary (bounds mode)")

	fs.Float64Var(&o.centerRe, "centerRe", 0.0, "center real (center mode)")
	fs.Float64Var(&o.centerIm, "centerIm", 0.0, "center imaginary (center mode)")
	fs.Float64Var(&o.mag, "mag", 1.0, "magnification, 1 = full default span (center mode)")

	fs.IntVar(&o.iter, "iter", 1000, "maximum iterations")
	fs.IntVar(&o.parallel, "parallel", 0, "worker goroutines (0 = NumCPU)")

	fs.StringVar(&o.paletteName, "palette", "rainbow", "grayscale|fire|blue|rainbow")
	fs.BoolVar(&o.smooth, "smooth", false, "enable continuous smooth colouring")
	fs.StringVar(&o.inside, "inside", "#000000", "in-set colour as #RRGGBB")

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Any centre flag switches to centre mode, ignoring the bounds.
	centerMode := wasSet(fs, "mag") || wasSet(fs, "centerRe") || wasSet(fs, "centerIm")

	cfg, err := buildConfig(o, centerMode)
	if err != nil {
		return err
	}

	if !centerMode && cfg.View.AspectMismatch() {
		fmt.Fprintln(stderr, "warning: viewport aspect ratio differs from image aspect ratio; image will be stretched")
	}

	img := render.Render(cfg)
	if err := imageio.EncodeFile(o.out, img, o.dpi); err != nil {
		return fmt.Errorf("writing %s: %w", o.out, err)
	}
	fmt.Fprintf(stderr, "wrote %s (%dx%d, iter=%d, palette=%s)\n",
		o.out, o.width, o.height, o.iter, o.paletteName)
	return nil
}

func buildConfig(o options, centerMode bool) (render.Config, error) {
	var zero render.Config
	if o.width <= 0 || o.height <= 0 {
		return zero, fmt.Errorf("width and height must be > 0")
	}
	if int64(o.width)*int64(o.height) > maxPixels {
		return zero, fmt.Errorf("image too large: %d*%d exceeds %d pixels", o.width, o.height, maxPixels)
	}
	if o.iter <= 0 {
		return zero, fmt.Errorf("iter must be > 0")
	}
	if o.dpi < 0 {
		return zero, fmt.Errorf("dpi must be >= 0")
	}

	var view viewport.Viewport
	if centerMode {
		if o.mag <= 0 {
			return zero, fmt.Errorf("mag must be > 0")
		}
		view = viewport.FromCenter(o.centerRe, o.centerIm, o.mag, o.width, o.height)
	} else {
		if o.minRe >= o.maxRe {
			return zero, fmt.Errorf("minRe (%g) must be < maxRe (%g)", o.minRe, o.maxRe)
		}
		if o.minIm >= o.maxIm {
			return zero, fmt.Errorf("minIm (%g) must be < maxIm (%g)", o.minIm, o.maxIm)
		}
		view = viewport.FromBounds(o.minRe, o.maxRe, o.minIm, o.maxIm, o.width, o.height)
	}

	inside, err := palette.ParseHexColor(o.inside)
	if err != nil {
		return zero, err
	}
	pal, err := palette.New(o.paletteName, inside, o.smooth)
	if err != nil {
		return zero, err
	}

	return render.Config{
		View:     view,
		Palette:  pal,
		MaxIter:  o.iter,
		Parallel: o.parallel,
	}, nil
}

// wasSet reports whether the flag was given explicitly.
func wasSet(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}
