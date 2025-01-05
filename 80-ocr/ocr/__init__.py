"""OCR by template matching against glyphs rendered with Pillow.

Usage::

    from ocr import OCREngine
    engine = OCREngine()
    result = engine.read("image.png")
    print(result.text)
"""

from __future__ import annotations

from .classify import TemplateBank, extract_features
from .glyphs import DEFAULT_CHARSET, get_font, render_glyph
from .pipeline import OCREngine
from .types import (
    Box,
    CharResult,
    NoTextFoundError,
    OCRError,
    OCRResult,
    UnreadableImageError,
)

__all__ = [
    "OCREngine",
    "OCRResult",
    "CharResult",
    "Box",
    "TemplateBank",
    "extract_features",
    "render_glyph",
    "get_font",
    "DEFAULT_CHARSET",
    "OCRError",
    "UnreadableImageError",
    "NoTextFoundError",
]

__version__ = "1.0.0"
