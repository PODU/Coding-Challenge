"""Shared test fixtures.

Every fixture is rendered at runtime with Pillow's bundled font — nothing is
downloaded, so the whole suite is fully offline and deterministic.
"""

from __future__ import annotations

import numpy as np
import pytest
from PIL import Image, ImageDraw

from ocr import OCREngine
from ocr.glyphs import get_font

# The default render size for fixtures. At this size the bundled font separates
# adjacent glyphs cleanly (some letters touch at smaller sizes), so segmentation
# is unambiguous. It is also one of the engine's template sizes.
DEFAULT_RENDER_SIZE = 48


def render_text_image(
    text: str,
    *,
    font_size: int = DEFAULT_RENDER_SIZE,
    angle: float = 0.0,
    fg: tuple[int, int, int] = (0, 0, 0),
    bg: tuple[int, int, int] = (255, 255, 255),
    pad: int = 16,
) -> Image.Image:
    """Rasterise *text* to an RGB image for round-trip OCR tests.

    The canvas is a full line-height box so a glyph's vertical position relative
    to the baseline is preserved (needed to tell e.g. ``o`` from ``O``).
    Multi-line text (``"\\n"``) is supported.
    """
    font = get_font(font_size)
    ascent, descent = font.getmetrics()
    lines = text.split("\n")
    probe = Image.new("L", (8, 8))
    draw = ImageDraw.Draw(probe)
    width = max((int(draw.textlength(ln, font=font)) for ln in lines), default=1)
    line_h = ascent + descent
    height = line_h * len(lines)
    img = Image.new("RGB", (width + 2 * pad, height + 2 * pad), bg)
    d = ImageDraw.Draw(img)
    for i, ln in enumerate(lines):
        d.text((pad, pad + i * line_h), ln, font=font, fill=fg)
    if angle:
        img = img.rotate(angle, expand=True, fillcolor=bg, resample=Image.BICUBIC)
    return img


@pytest.fixture(scope="session")
def engine() -> OCREngine:
    """A shared OCR engine (the template bank is built once per session)."""
    return OCREngine()


@pytest.fixture(scope="session")
def render():
    """Expose the renderer as a fixture."""
    return render_text_image


def levenshtein(a: str, b: str) -> int:
    """Classic edit distance (for character-accuracy assertions)."""
    m, n = len(a), len(b)
    prev = list(range(n + 1))
    for i in range(1, m + 1):
        cur = [i] + [0] * n
        for j in range(1, n + 1):
            cost = 0 if a[i - 1] == b[j - 1] else 1
            cur[j] = min(prev[j] + 1, cur[j - 1] + 1, prev[j - 1] + cost)
        prev = cur
    return prev[n]


def char_accuracy(expected: str, got: str) -> float:
    """1 - normalised edit distance, in [0, 1]."""
    if not expected:
        return 1.0 if not got else 0.0
    return 1.0 - levenshtein(expected, got) / len(expected)


def add_noise(img: Image.Image, amount: int = 35, seed: int = 0) -> Image.Image:
    """Add uniform pixel noise to an image (robustness fixture)."""
    arr = np.asarray(img).astype(np.int16)
    rng = np.random.default_rng(seed)
    arr = arr + rng.integers(-amount, amount + 1, arr.shape, dtype=np.int16)
    return Image.fromarray(np.clip(arr, 0, 255).astype(np.uint8))
