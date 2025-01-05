"""Render glyphs with the font bundled in Pillow.

``ImageFont.load_default(size)`` returns a scalable TrueType font since Pillow
10.1, so the templates and the test images come from the same font and nothing
needs downloading.
"""

from __future__ import annotations

from functools import lru_cache

import numpy as np
from PIL import Image, ImageDraw, ImageFont

from .detect import true_runs

DEFAULT_CHARSET = (
    "0123456789"
    "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
    "abcdefghijklmnopqrstuvwxyz"
    ".,:-"
)


@lru_cache(maxsize=32)
def get_font(size: int) -> ImageFont.FreeTypeFont:
    """Pillow's bundled font at *size* pixels."""
    font = ImageFont.load_default(size=size)
    if not isinstance(font, ImageFont.FreeTypeFont):  # pragma: no cover
        raise RuntimeError(
            "ImageFont.load_default(size) did not return a scalable font; "
            "Pillow >= 10.1 is required."
        )
    return font


def font_metrics(font: ImageFont.FreeTypeFont) -> tuple[int, int, int]:
    """Return ``(ascent, descent, cap_top)``, where ``cap_top`` is the top row
    of a capital H drawn at y=0."""
    ascent, descent = font.getmetrics()
    cap_top = int(font.getbbox("H")[1])
    return int(ascent), int(descent), cap_top


def cap_height_ratio(font: ImageFont.FreeTypeFont, size: int, charset: str) -> float:
    """Height from the tallest glyph top to the baseline, as a fraction of size.

    Dividing a line's measured cap height by this estimates its font size.
    """
    ascent, _descent = font.getmetrics()
    tops = [font.getbbox(ch)[1] for ch in charset if ch.strip()]
    ascender_top = min(tops) if tops else 0
    return (ascent - ascender_top) / float(size)


def natural_gap(font: ImageFont.FreeTypeFont, a: str, b: str) -> int:
    """Pixel gap between the ink of ``a`` and ``b`` drawn next to each other,
    with no space between them. Rendering the pair picks up the real advance
    widths and side bearings."""
    text = a + b
    length = int(font.getlength(text)) + 12
    ascent, descent = font.getmetrics()
    img = Image.new("L", (max(2, length), ascent + descent), 0)
    ImageDraw.Draw(img).text((4, 0), text, fill=255, font=font)
    runs = true_runs((np.asarray(img) > 127).any(axis=0))
    if len(runs) < 2:
        return 0
    return runs[1][0] - runs[0][1]


def render_glyph(ch: str, font: ImageFont.FreeTypeFont) -> np.ndarray:
    """Draw *ch* on a canvas one line high and return its ink mask.

    Keeping the full line height preserves where the glyph sits relative to the
    baseline, which is what tells ``o`` from ``O``.
    """
    ascent, descent = font.getmetrics()
    height = ascent + descent
    length = font.getlength(ch)
    # Leave room for side bearings.
    pad = 6
    width = max(1, int(round(length)) + 2 * pad)
    img = Image.new("L", (width, height), 0)
    draw = ImageDraw.Draw(img)
    draw.text((pad, 0), ch, fill=255, font=font)
    return np.asarray(img) > 127
