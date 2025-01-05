"""The OCR engine: load, binarise, deskew, segment, classify."""

from __future__ import annotations

import numpy as np

from . import deskew as deskew_mod
from . import detect as detect_mod
from . import glyphs as glyphs_mod
from . import segment as segment_mod
from .classify import TemplateBank
from .glyphs import DEFAULT_CHARSET
from .loader import ImageInput, load_gray
from .preprocess import binarize
from .types import Box, CharResult, OCRResult

# A gap this much wider than the pair's natural gap, as a fraction of the
# width of a space, is a word break.
_SPACE_FRACTION = 0.45


class OCREngine:
    """Reads text from images by matching glyphs against rendered templates."""

    def __init__(
        self,
        charset: str = DEFAULT_CHARSET,
        *,
        font_size: int = 32,
        font_sizes: tuple[int, ...] | None = None,
        min_confidence: float = 0.40,
        auto_deskew: bool = True,
    ) -> None:
        self.charset = charset
        self.min_confidence = min_confidence
        self.auto_deskew = auto_deskew
        # Glyphs rasterise slightly differently at each size, so templates at
        # several sizes match text better across that range.
        sizes = (
            font_sizes
            if font_sizes is not None
            else (20, 24, 28, 32, 36, 40, 44, 48, 56, 64)
        )
        self.bank = TemplateBank(charset=charset, font_sizes=sizes)
        # Used to estimate each line's font size from its cap height.
        ref = 32
        self._cap_ratio = glyphs_mod.cap_height_ratio(
            glyphs_mod.get_font(ref), ref, charset
        )
        self._gap_cache: dict[tuple[int, str, str], int] = {}

    def read(self, image: ImageInput) -> OCRResult:
        """Recognise the text in *image*."""
        gray = load_gray(image)
        mask = binarize(gray)

        skew_angle = 0.0
        if self.auto_deskew and mask.any():
            angle = deskew_mod.estimate_skew(mask)
            if abs(angle) >= 0.3:
                gray = deskew_mod.deskew(gray, angle)
                mask = binarize(gray)
                skew_angle = angle

        regions = detect_mod.find_text_regions(mask)
        if not regions:
            return OCRResult(text="", chars=[], regions=[], skew_angle=skew_angle)

        lines = segment_mod.segment(mask, mark_word_spaces=False)
        chars: list[CharResult] = []
        line_texts: list[str] = []

        for ln in lines:
            line_mask = mask[ln.y0 : ln.y1]
            recognised: list[tuple[str, float, tuple[int, int]]] = []
            for sp in ln.spans:
                x0, x1 = sp.x0, sp.x1
                ch, conf = self.bank.recognize_strip(
                    line_mask, x0, x1, ln.baseline, ln.cap_top
                )
                if conf < self.min_confidence:
                    ch = "?"
                recognised.append((ch, conf, (x0, x1)))
                box = Box(x0, ln.y0, x1 - x0, ln.y1 - ln.y0)
                chars.append(CharResult(char=ch, box=box, confidence=conf))

            line_texts.append(self._compose_line(recognised, ln.baseline - ln.cap_top))

        return OCRResult(
            text="\n".join(line_texts),
            chars=chars,
            regions=regions,
            skew_angle=skew_angle,
        )

    def _compose_line(
        self,
        recognised: list[tuple[str, float, tuple[int, int]]],
        cap_height: float,
    ) -> str:
        """Join a line's characters, adding spaces where gaps are too wide.

        A gap is compared with how far apart the same two characters would be
        drawn without a space.
        """
        if not recognised:
            return ""
        est_size = int(round(cap_height / self._cap_ratio)) if self._cap_ratio else 32
        est_size = max(8, min(200, est_size))
        font = glyphs_mod.get_font(est_size)
        space_adv = font.getlength(" ")
        out = [recognised[0][0]]
        for (pa, _, (_, ax1)), (pb, _, (bx0, _)) in zip(recognised, recognised[1:]):
            gap = bx0 - ax1
            if pa == "?" or pb == "?":
                is_space = gap >= segment_mod.SPACE_RATIO * cap_height
            else:
                natural = self._natural_gap(est_size, font, pa, pb)
                is_space = (gap - natural) > _SPACE_FRACTION * space_adv
            if is_space:
                out.append(" ")
            out.append(pb)
        return "".join(out).rstrip()

    def _natural_gap(self, size: int, font, a: str, b: str) -> int:
        key = (size, a, b)
        cached = self._gap_cache.get(key)
        if cached is None:
            cached = glyphs_mod.natural_gap(font, a, b)
            self._gap_cache[key] = cached
        return cached
