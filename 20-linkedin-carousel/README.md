# LinkedIn Carousel Generator

> Coding Challenges #20 — turn Markdown / plain text into a multi-slide,
> square **PDF carousel** ready to post on LinkedIn.

Content in, polished PDF out — no graphic-design skills, no headless browser,
no network. Every page is a `1080×1080` (configurable up to `4320×4320`)
square. Rendering is pure-library (**reportlab** for vector PDF, **Pillow** for
image compositing), so the whole pipeline is deterministic and fully testable
offline.

## Features

- **Markdown/text → multi-page PDF**, one page per slide.
- **Global styling**: background color, foreground/text color, font face
  (core font or your own `.ttf`), base font size.
- **Profile identity** top-left: circular profile picture + name / handle.
- **Per-slide background images** with configurable **opacity** (cover-cropped
  to square, alpha-blended over the background color).
- **Auto-layout**: long text wraps to the content width, auto-shrinks the font
  to a floor, and truncates with an ellipsis only if it still overflows. Long
  URLs / single tokens hard-break.
- **Graceful degradation**: missing fonts fall back to Helvetica; missing or
  unreadable images warn to stderr and are skipped — the PDF still renders.
- **`n / N` page indicators** (toggleable).

## Setup

Requires Python 3.10+.

```bash
# from the 20-linkedin-carousel/ directory
python -m pip install -e .          # runtime: reportlab, pillow
python -m pip install -e ".[test]"  # + test deps: pytest, pypdf
```

Or install the pinned set directly:

```bash
python -m pip install -r requirements.txt
```

## Run

The tool is exposed both as a console script (`carousel`) and as a module
(`python -m carousel` — handy if the Scripts directory isn't on your `PATH`):

```bash
carousel examples/post.md -o post.pdf
# equivalently:
python -m carousel examples/post.md -o post.pdf
```

### CLI

```
carousel INPUT [-o OUTPUT.pdf] [options]

Positional:
  INPUT                     Markdown/text carousel source file.

Options:
  -o, --output PATH         Output PDF. Default: <input stem>.pdf
  -c, --config PATH         Style/profile JSON. Default: style.json beside the
                            input, if present.
  --size N                  Square slide size in px (default 1080, max 4320).
  --bg COLOR                Background color (name or #RRGGBB).
  --fg COLOR                Foreground/text color.
  --font NAME               Font face (core name or path to a .ttf).
  --font-size N             Base body font size in px.
  --profile-image PATH      Profile picture (top-left).
  --name TEXT               Display name.
  --handle TEXT             Handle, e.g. @janedev.
  --slide-separator STR     Slide delimiter (default: a line equal to '---').
  --page-numbers / --no-page-numbers
  -q, --quiet               Suppress warnings.
  -h, --help
  --version
```

Precedence: **CLI flags > config file > built-in defaults.** Relative image
paths are resolved against the input file's directory.

**Exit codes:** `0` success · `1` bad input (missing file, parse/render error)
· `2` usage error (argparse).

## Input format

Slides are separated by a horizontal rule (`---`). A leading `# Heading`
becomes the slide title; the rest is body text. Per-slide background images use
an HTML-comment directive:

```markdown
# Slide one title
Body text for the first slide. It wraps and auto-shrinks to fit.

---

<!-- bg: beach.jpg opacity: 0.3 -->
# Slide two
This slide has a background image at 30% opacity.
```

### Styling via `style.json`

A sidecar JSON file (auto-discovered as `style.json` next to the input, or
passed with `-c`) supplies global style + profile:

```json
{
  "style": {
    "background": "#0A1F44",
    "foreground": "#FFFFFF",
    "font": "fonts/Inter-Regular.ttf",
    "font_size": 56,
    "size": 1080
  },
  "profile": {
    "image": "me.png",
    "name": "Jane Dev",
    "handle": "@janedev"
  }
}
```

## Examples

```bash
# Use the bundled example deck + style
carousel examples/post.md -o examples/post.pdf -c examples/style.json

# Override styling on the command line (wins over the config file)
carousel examples/post.md -o post.pdf \
  --bg "#0A1F44" --fg white \
  --font-size 56 --name "Jane Dev" --handle "@janedev"

# A custom TTF, larger canvas, no page numbers
carousel post.md --font fonts/Inter-Regular.ttf --size 1440 --no-page-numbers
```

## Library use

```python
from carousel import build_carousel, render_carousel, Style, Profile

carousel = build_carousel(
    open("post.md", encoding="utf-8").read(),
    style=Style(background="#0A1F44", foreground="white", font_size=56),
    profile=Profile(name="Jane Dev", handle="@janedev"),
)
render_carousel(carousel, "post.pdf")
```

## Tests

```bash
python -m pytest
```

The suite (42 tests) runs offline with no committed binary assets — fixtures
generate small PNGs and Markdown docs in `tmp_path`. It asserts:

- a valid PDF is produced (`%PDF-` header) and **page count == slide count**
  for several N;
- background + profile images render without exception (file size > 0);
- a **missing optional image** warns and still produces a correct PDF;
- **long text** wraps, auto-shrinks, and truncates without crashing;
- rendering touches **no network** (a socket guard fails the test if it does);
- empty input exits `1` with a message and leaves **no partial PDF**;
- color/opacity/size parsing, clamping, and config-merge precedence.

## Project layout

```
20-linkedin-carousel/
  README.md
  pyproject.toml          # console script: carousel = carousel.cli:main
  requirements.txt
  src/carousel/
    __init__.py
    __main__.py           # python -m carousel
    models.py             # Style, Profile, Slide, Carousel, parse_color
    parser.py             # Markdown/text + JSON config -> Carousel
    layout.py             # pure geometry: wrap, auto-shrink, ellipsis
    renderer.py           # reportlab PDF + Pillow image compositing
    cli.py                # argument parsing, wiring, exit codes
  examples/
    post.md
    style.json
  tests/
    conftest.py
    test_parser.py
    test_layout.py
    test_renderer.py
    test_cli.py
```

## Design notes & caveats

- **Why not a headless browser?** Puppeteer/Chromium is heavy, binary- and
  network-dependent, and flaky in offline CI. reportlab+Pillow give exact-pixel
  pages and ergonomic image compositing as pure-Python wheels.
- **Fonts**: core PDF fonts (Helvetica, Times-Roman, Courier, …) embed nothing
  and are always available. Supply a `.ttf`/`.otf` path for custom faces; it is
  embedded via `pdfmetrics`. Title text uses the same face at a larger size
  (no separate bold variant is assumed for custom TTFs).
- **Non-ASCII / emoji**: render correctly with a Unicode TTF; with a core font,
  unsupported glyphs degrade gracefully (no crash).
- **Determinism**: identical input yields identical page count and extracted
  text. Raw PDF bytes are *not* byte-identical because reportlab embeds a
  creation timestamp / document ID; tests compare structure + extracted text.
- **Image paths** are resolved relative to the input file's directory (CLI) or
  used as-is (library API).
- The console script `carousel` is installed into your Python *Scripts*
  directory; if that isn't on `PATH` (common with Microsoft Store Python), use
  `python -m carousel`.
```
