# Coding Challenge #80 — OCR

A small, **offline, deterministic** Optical Character Recognition engine built on
**Pillow + NumPy only** — no Tesseract, no neural-network weights, nothing
downloaded at runtime or test time.

It implements the classic OCR pipeline from the challenge:

1. **Detect** text and locate it (binarise + connected components / projection).
2. **De-skew** so the baseline is horizontal (projection-variance search).
3. **Segment** the image into lines → words → individual characters.
4. **Recognise** each glyph by **template / matrix matching** against glyphs the
   engine renders itself with Pillow's bundled font.

Because the "training data" is rasterised on demand from a font, the classifier
and the test fixtures share one source of truth, the output is repeatable, and
the whole suite runs in ~1.5 s with zero network access.

---

## Install

```bash
cd 80-ocr
python -m pip install -r requirements.txt   # pillow, numpy, pytest
# or install the package (provides the `ocr` console script):
python -m pip install -e .
```

Requires Python ≥ 3.10 and Pillow ≥ 10.1 (for the scalable bundled default
font). OpenCV is **not** required.

---

## Command-line usage

```bash
# Print recognised text
python -m ocr.cli image.png
ocr image.png                      # if installed as a package

# Structured JSON (text + per-character boxes + confidences)
python -m ocr.cli image.png --json

# Write the input with character boxes drawn (Step 3 visual proof)
python -m ocr.cli image.png --boxes boxes.png

# Tunables
python -m ocr.cli image.png --min-conf 0.5 --no-deskew --charset "0123456789"
```

## Library usage

```python
from ocr import OCREngine

engine = OCREngine()                 # builds the template bank once
result = engine.read("invoice.png")  # path | bytes | PIL.Image | np.ndarray

print(result.text)                   # "Invoice 2024"
for ch in result.chars:
    ch.char, ch.box, ch.confidence   # str, Box(x,y,w,h), float 0..1

result.to_json()                     # structured output
result.draw_boxes(base_image)        # PIL.Image overlay of character boxes
result.skew_angle                    # degrees corrected
result.regions                       # detected text-line boxes (Step 1)
```

An empty or text-less image returns `OCRResult(text="", chars=[], regions=[])`
rather than raising, so the common case needs no `try/except`.

---

## How it works

| Stage | Module | Notes |
|-------|--------|-------|
| Load | `loader.py` | path / bytes / `PIL.Image` / `np.ndarray` → grayscale array |
| Preprocess | `preprocess.py` | luminosity grayscale, **Otsu** global threshold (pure NumPy), adaptive local-mean fallback, automatic **polarity** detection (black-on-white *and* white-on-black) |
| Detect | `detect.py` | connected components + row/column projection → line region boxes |
| Deskew | `deskew.py` | search ±15°, maximise horizontal-projection variance; rotate with Pillow |
| Segment | `segment.py` | rows → lines; connected components → characters (merging `i`/`j` dots and the two dots of `:`) |
| Recognise | `classify.py` | per-glyph **shape** descriptor + **size/position ratios**, nearest-neighbour by cosine similarity |
| Spacing | `pipeline.py` | two-pass: classify first, then insert word spaces using the font's exact glyph geometry of the recognised neighbours |

### Why two descriptors for recognition?

A glyph is described by two decoupled parts so that font size, vertical offset
and **letter case** are handled independently:

* a **shape** grid — the glyph cropped to its own ink box and resized into a
  fixed grid. This is translation- and scale-invariant, so `o` and `O` share
  (almost) the same shape and a glyph's identity is captured robustly.
* **size/position ratios** — height and top/bottom extent relative to the line's
  cap-height. These are the *only* things that distinguish case (`o` vs `O`) and
  vertical role (`.` vs `-` vs `'`), and they tolerate a couple of pixels of
  baseline/cap uncertainty.

### Word spacing

Word gaps are decided **after** recognition: for each pair of recognised
characters the engine renders that exact pair with the font to obtain the gap
expected with *no* space, and flags a word break only when the observed gap
exceeds it by a fraction of the font's space advance. This replicates Pillow's
real text layout, so it copes with glyphs that have wide side-bearings (e.g. the
gap after `1`) where a single fixed threshold cannot.

---

## Tests

```bash
python -m pytest          # 57 tests, ~1.5 s, fully offline
```

The suite renders known text to images with Pillow and OCRs it back, asserting
the decoded string. It covers binarisation/Otsu/polarity, detection (box IoU at
several positions), deskew recovery, line/word/character segmentation, the
classifier, and end-to-end round trips plus robustness variants (rotation,
colour, white-on-black, noise, cross-size, font-size invariance, empty image,
idempotence) and the result API (`to_json`, `draw_boxes`, numpy/path inputs).

### Achieved accuracy

Measured on rendered Latin text (digits, upper- and lower-case, and `.,:-`):

| Condition | Character accuracy | Exact-string |
|-----------|-------------------|--------------|
| Default render size (48 px) | **100%** | 12 / 12 |
| Across template sizes (32–56 px) | 99.3% | 45 / 48 |
| Non-template sizes (30–52 px) | 99.0% | 65 / 72 |
| Rotated ±3–8° (deskew) | 100% | — |

Exact-string round-trips include digits, `HELLO WORLD`, the full upper- and
lower-case alphabets, `The quick brown Fox 123`, the pangram *“The quick brown
fox jumps over the lazy dog”*, punctuation (`a,b:c-d.`, `Hello, World.`) and
multi-word spacing.

---

## Known limitations / caveats

* **Touching glyphs at small sizes.** With the bundled font, some letters
  (`v w x y`) merge into one ink blob below ~40 px, so the full lower-case
  alphabet should be read at ≥ 40 px (fixtures render at 48 px). Connected
  components separate them at larger sizes.
* **`l` / `I` / `1` and `O` / `0`.** These are near-identical in the bundled
  font and may be confused at sizes far from a template size — the classic OCR
  ambiguity. Accuracy is exact at template sizes.
* **Pathological spacing.** A few glyph pairs render a word-space narrower than
  another pair's intra-word side-bearing; pure geometry cannot separate every
  such case (real OCR uses a language model). The two-pass geometry method
  handles all common prose and numeric spacing.
* **Scope.** v1 targets clean, printed, single-font Latin text rendered by the
  same font family used for the templates (the challenge's own test material).
  Cursive/handwriting, arbitrary photographs, dense multi-column layout and
  non-Latin scripts are out of scope. The `charset` is configurable.

---

## Project layout

```
80-ocr/
├── ocr/
│   ├── loader.py       # image input -> grayscale array
│   ├── preprocess.py   # grayscale, Otsu/adaptive binarise, polarity
│   ├── detect.py       # connected components, text-region boxes
│   ├── deskew.py       # skew estimation + correction
│   ├── segment.py      # lines / words / characters
│   ├── glyphs.py       # Pillow glyph rendering + font geometry
│   ├── classify.py     # TemplateBank, feature extraction, NN recognition
│   ├── pipeline.py     # OCREngine orchestration + word spacing
│   ├── types.py        # Box, CharResult, OCRResult, errors
│   └── cli.py          # argparse CLI
├── tests/              # offline pytest suite (renders fixtures at runtime)
├── pyproject.toml      # packaging + `ocr` console script
├── requirements.txt
```
