# Mandelbrot Set — Coding Challenge #84

A command-line tool that computes and renders the [Mandelbrot set](https://en.wikipedia.org/wiki/Mandelbrot_set)
to a PNG image. For each pixel it maps the position to a point `c` on the complex
plane, runs the escape-time algorithm

```
z_{n+1} = z_n^2 + c,   z_0 = 0
```

(iterating until `|z| > 2` or a maximum iteration count is reached), and maps the
resulting iteration count to a colour.

Pure Go, **standard library only** — no third-party dependencies.

## Build & Run

```sh
cd 84-mandelbrot-set
go build ./cmd/mandelbrot
./mandelbrot                       # classic full-set view -> mandelbrot.png
go test ./...                      # run the test suite
```

## Usage

```
mandelbrot [flags]

Image:
  -width int        Output width in pixels             (default 800)
  -height int       Output height in pixels            (default 800)
  -out string       Output PNG path                    (default "mandelbrot.png")
  -dpi int          DPI metadata for print (pHYs chunk) (default 72)

Region — Bounds mode (R3a):
  -minRe float      Min real                           (default -2.0)
  -maxRe float      Max real                           (default  2.0)
  -minIm float      Min imaginary                      (default -2.0)
  -maxIm float      Max imaginary                      (default  2.0)

Region — Center mode (R3b), overrides bounds mode when any is set:
  -centerRe float   Center real coordinate
  -centerIm float   Center imaginary coordinate
  -mag float        Magnification (1 = full default span)  (default 1.0)

Compute:
  -iter int         Maximum iterations                 (default 1000)
  -parallel int     Worker goroutines (0 = NumCPU)      (default 0)

Colour (R4):
  -palette string   grayscale|fire|blue|rainbow        (default "rainbow")
  -smooth           Continuous smooth colouring         (default false)
  -inside string    In-set colour as #RRGGBB           (default "#000000")
```

### Examples

```sh
# Default full-set render
./mandelbrot

# Zoom by explicit bounds
./mandelbrot -minRe -0.75 -maxRe -0.70 -minIm 0.10 -maxIm 0.15 -iter 2000

# Center + magnification (Seahorse Valley) with smooth fire palette
./mandelbrot -centerRe -0.743 -centerIm 0.131 -mag 200 -palette fire -smooth \
             -width 1920 -height 1080 -iter 3000 -out seahorse.png

# Print poster at 300 DPI
./mandelbrot -width 6000 -height 6000 -dpi 300 -iter 5000 -out poster.png
```

## Architecture

```
cmd/mandelbrot/main.go   CLI parsing, validation, wiring, image write
internal/fractal/        escape-time core + cardioid/bulb early-out + smooth iteration
internal/viewport/       complex-plane <-> pixel mapping (bounds & center+mag)
internal/palette/        iteration-count -> color.RGBA palettes
internal/render/         parallel scanline renderer -> image.RGBA
internal/imageio/        PNG writer with DPI (pHYs chunk) metadata
```

Each layer is pure and independently testable.

- **Escape-time core.** Real-arithmetic iteration comparing squared magnitude to
  the squared escape radius (no `sqrt`). Escape radius is `2^8` for clean smooth
  colouring. Points provably inside the main cardioid or the period-2 bulb are
  skipped via an exact algebraic test.
- **Viewport.** `FromBounds` for an explicit box; `FromCenter` derives the
  imaginary span from the real span × `height/width` so pixels stay square.
- **Palette.** `grayscale`, `fire`, `blue`, `rainbow` (HSV sweep). In-set points
  use a configurable colour. `-smooth` applies `mu = iters + 1 - log2(log|z|)` to
  remove banding.
- **Renderer.** A worker pool of goroutines consumes whole scanlines; every pixel
  is a pure function of its coordinate, so output is byte-identical regardless of
  worker count or scheduling. Workers write disjoint `Pix` ranges (no locking).
- **PNG writer.** Uses `image/png`, then splices a `pHYs` chunk after `IHDR` to
  carry physical resolution (`dpi -> round(dpi * 39.3701)` pixels per metre),
  which the stdlib encoder does not expose.

## Notes & Caveats

- **The rendered set is an approximation** parameterized by `-iter`. Boundary
  points need high iteration counts to classify; low counts make the set look
  "fatter" than it is.
- **Deep-zoom precision.** `float64` carries ~15–16 significant digits; beyond
  ~`10^13`–`10^14` magnification adjacent pixels round to the same `c` and the
  image pixelates. Arbitrary precision / perturbation theory is out of scope (v1).
- **Aspect ratio.** Center mode keeps pixels square automatically. Bounds mode
  prints a warning if the plane rectangle's aspect ratio differs from the image's.
- **Very large images** are bounds-checked (`width*height <= 400,000,000`) to fail
  cleanly rather than OOM.
- **Determinism (NF2)** is verified by tests; `parallel=1` output is identical to
  multi-worker output.

## Tests

```sh
go test ./...
```

Covers: escape-time for known points (`c=0`/`c=-1` in set, `c=2`/`c=1` escape,
off-axis escape, cardioid early-out vs. brute force, monotonicity), viewport
corner mapping and plane↔pixel round-trip, square-pixel invariant, render
dimensions, in-set centre pixel / escaping corner pixel, determinism,
parallel == serial output, PNG encode/decode round-trip, and `pHYs`/DPI metadata,
plus CLI validation and bad-flag handling.

> `go test -race` requires cgo; on a Windows box without a 64-bit gcc the race
> build cannot link. The renderer is race-free by construction (workers write
> disjoint pixel ranges), and parallel-vs-serial equality is asserted by test.
