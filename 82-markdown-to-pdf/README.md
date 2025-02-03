# md2pdf — Markdown to PDF Converter

> [Coding Challenges #82](https://codingchallenges.substack.com/p/coding-challenge-82-markdown-to-pdf)

A command-line tool (and small library) that converts a Markdown file into a
**paginated PDF**. It uses a **pure-Python, offline** route — `markdown-it-py`
for CommonMark parsing and `reportlab` for PDF layout — so there is **no
headless browser**, no Chromium, no network, and no system dependencies beyond
Python and two pip packages.

## Features

Renders all of the required Markdown elements:

- Headings `#` … `######` (distinct, decreasing, bold sizes)
- Paragraphs with wrapped body text
- **Bold** (`**`/`__`), *italic* (`*`/`_`), and ***bold italic*** (`***`)
- Unordered lists (`-`, `+`, `*`) and ordered lists (`1.`), with **nesting**
- `inline code` and fenced/indented ```code blocks``` (shaded, whitespace preserved)
- Links — coloured, underlined, and **clickable** PDF annotations
- Images — embedded and scaled to the content width; alt-text fallback if missing

Page layout:

- **A4** (default) or **Letter** page size
- Configurable margins (default 2 cm)
- **Automatic pagination** — long content flows onto more pages
- Footer page numbers: `Page N of M`
- Code blocks / images that don't fit move to the next page rather than clip
- Long URLs / code lines wrap instead of overflowing the margin

Robustness: malformed Markdown, empty files, missing images, and unsupported
constructs (tables, blockquotes, raw HTML) degrade gracefully — never a crash.

## Installation

```bash
python -m pip install -e .
# or, just the runtime deps:
python -m pip install markdown-it-py reportlab
```

This installs an `md2pdf` console script. You can also run it as a module
(`python -m md2pdf`), which always works regardless of your `PATH`.

## Usage

```bash
md2pdf README.md                          # -> README.pdf, A4 defaults
md2pdf notes.md -o out/notes.pdf --page-size Letter
md2pdf spec.md --font Times-Roman --font-size 12 --heading-scale 1.15
cat doc.md | md2pdf - -o doc.pdf          # read Markdown from stdin
```

### Options

| Flag | Default | Description |
|------|---------|-------------|
| `-o, --output PATH` | `INPUT.pdf` | Output PDF path |
| `--page-size {A4,Letter}` | `A4` | Page size |
| `--margin MM` | `20` | Page margin in millimetres |
| `--font NAME` | `Helvetica` | Body font: `Helvetica`, `Times-Roman`, `Courier`, or a `.ttf`/`.otf` path |
| `--font-size PT` | `11` | Base body font size |
| `--heading-scale FLOAT` | `1.0` | Multiplier applied to all heading sizes |
| `--background HEX` | `#ffffff` | Page background colour |
| `--link-color HEX` | `#1a0dab` | Link text colour |
| `--code-bg HEX` | `#f2f2f2` | Code background colour |
| `--title TEXT` | – | Document title for PDF metadata |
| `--allow-remote-images` | off | Download `http(s)` image URLs (kept off to stay offline) |
| `-q, --quiet` | – | Suppress progress output |
| `--version`, `-h/--help` | – | Version / help |

**Exit codes:** `0` success · `1` usage error (argparse) · `2` processing
error (unreadable input / unwritable output) — reported on stderr, never a
traceback.

## Library API

```python
from md2pdf import convert_markdown, convert_file, Style

pdf_bytes = convert_markdown("# Hello\n\nWorld.", Style(page_size="Letter"))
convert_file("notes.md", "notes.pdf", Style(font_size=12))
```

The pipeline is three decoupled stages:

```
source.md → parse (markdown-it-py)
          → model.py   : token stream → typed Document IR (no reportlab)
          → render.py  : Document + Style → reportlab flowables → PDF bytes
```

The `Document` IR carries no PDF dependency, so parsing is unit-testable on its
own and an alternate (HTML/Word) Stage-3 backend remains a drop-in.

## Examples

- `examples/sample.md` — exercises every supported element
- `examples/long.md` — multi-page document used for pagination tests

```bash
md2pdf examples/sample.md
```

## Testing

```bash
python -m pip install -e ".[test]"   # adds pytest + pypdf
python -m pytest
```

The suite (`tests/`) is fully **offline and deterministic**. It validates PDFs
both structurally (starts with `%PDF-`, contains `%%EOF`, re-parses with
`pypdf`) and semantically (extracted text contains the expected content), covers
every element type R1–R11, asserts multi-page pagination and the `Page N of M`
footer, checks both page sizes, exercises the CLI exit codes, and includes an
offline guarantee test that fails the render if any network call is attempted.

## Notes & caveats

- **Fonts:** the three built-in PDF families (`Helvetica`, `Times-Roman`,
  `Courier`) need zero assets. Passing a `.ttf`/`.otf` path to `--font`
  registers it (the single face is reused for bold/italic).
- **Remote images** are skipped by default (alt-text shown) to keep runs offline
  and deterministic; enable with `--allow-remote-images`.
- **Out of scope (v1):** tables, blockquotes (degraded to paragraphs),
  footnotes, task lists, strikethrough, syntax highlighting, HTML pass-through.
