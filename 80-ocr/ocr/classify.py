"""Classify glyphs by nearest match against rendered templates.

Each glyph's feature vector has two parts. The shape part is the glyph cropped
to its ink and scaled into a fixed grid, so it doesn't depend on size or
position; ``o`` and ``O`` look almost the same here. The second part is a few
ratios against the line's cap height (height, how far above and below the
baseline it reaches, aspect). Those are what separate ``o`` from ``O`` and
``.`` from ``-``.

Matching picks the template with the highest cosine similarity.
"""

from __future__ import annotations

import numpy as np
from PIL import Image

from .glyphs import (
    DEFAULT_CHARSET,
    font_metrics,
    get_font,
    render_glyph,
)

GRID = 32
_ZONE = 6  # size of the coarse density grid
_ZONE_W = 2.5
_PROFILE_W = 1.5
# Weights for the position features: strong enough to separate upper and lower
# case (whose shapes are nearly identical) without swamping real shape
# differences.
_POS_W = 5.0
_ASPECT_W = 4.0
_DENSITY_W = 2.0


def _resize_mask(mask: np.ndarray, new_w: int, new_h: int) -> np.ndarray:
    """Bilinear resize of a mask to ``(new_h, new_w)``, returning floats."""
    new_w = max(1, int(new_w))
    new_h = max(1, int(new_h))
    img = Image.fromarray((np.asarray(mask, dtype=np.float32) * 255).astype(np.uint8))
    img = img.resize((new_w, new_h), Image.BILINEAR)
    return np.asarray(img, dtype=np.float32) / 255.0


def _shape_grid(crop: np.ndarray) -> np.ndarray:
    """Scale *crop* to fit a GRID x GRID grid, centred, keeping its aspect."""
    ch, cw = crop.shape
    if ch == 0 or cw == 0:
        return np.zeros((GRID, GRID), np.float32)
    scale = (GRID - 2) / max(ch, cw)
    rh = max(1, int(round(ch * scale)))
    rw = max(1, int(round(cw * scale)))
    res = _resize_mask(crop, rw, rh)
    grid = np.zeros((GRID, GRID), np.float32)
    top = (GRID - rh) // 2
    left = (GRID - rw) // 2
    grid[top : top + rh, left : left + rw] = res
    return grid


def extract_features(
    line_mask: np.ndarray,
    x0: int,
    x1: int,
    baseline: float,
    cap_top: float,
    row_shift: int = 0,  # accepted for API compatibility; unused
) -> np.ndarray:
    """Feature vector for the glyph in columns ``[x0, x1)`` of ``line_mask``.

    ``baseline`` and ``cap_top`` are rows within ``line_mask``.
    """
    char = np.asarray(line_mask[:, x0:x1], dtype=bool)
    h, w = char.shape
    rows = np.where(char.any(axis=1))[0]
    cols = np.where(char.any(axis=0))[0]
    if rows.size == 0 or cols.size == 0:
        return _finalise(np.zeros((GRID, GRID), np.float32), 0.0, 0.0, 0.0, 0.0, 0.0)

    rtop, rbot = int(rows[0]), int(rows[-1])
    cleft, cright = int(cols[0]), int(cols[-1])
    crop = char[rtop : rbot + 1, cleft : cright + 1].astype(np.float32)

    grid = _shape_grid(crop)

    cap_h = baseline - cap_top
    if cap_h <= 1:
        cap_h = float(h)
    ink_h = rbot - rtop + 1
    ink_w = cright - cleft + 1
    rel_height = ink_h / cap_h
    rel_top = (baseline - rtop) / cap_h
    rel_bottom = (baseline - (rbot + 1)) / cap_h  # negative for descenders
    aspect = ink_w / float(ink_h)
    density = float(crop.mean())
    return _finalise(grid, rel_height, rel_top, rel_bottom, aspect, density)


def _zoning(grid: np.ndarray, z: int) -> np.ndarray:
    """Average-pool *grid* down to ``z x z``."""
    h, w = grid.shape
    ys = np.arange(h) * z // h
    xs = np.arange(w) * z // w
    out = np.zeros((z, z), np.float32)
    cnt = np.zeros((z, z), np.float32)
    np.add.at(out, (ys[:, None], xs[None, :]), grid)
    np.add.at(cnt, (ys[:, None], xs[None, :]), 1.0)
    return out / np.maximum(cnt, 1.0)


def _finalise(
    grid: np.ndarray,
    rel_height: float,
    rel_top: float,
    rel_bottom: float,
    aspect: float,
    density: float,
) -> np.ndarray:
    row_prof = grid.sum(axis=1) / GRID
    col_prof = grid.sum(axis=0) / GRID
    zones = _zoning(grid, _ZONE).ravel()
    vec = np.concatenate(
        [
            grid.ravel(),
            zones * _ZONE_W,
            row_prof * _PROFILE_W,
            col_prof * _PROFILE_W,
            [
                rel_height * _POS_W,
                rel_top * _POS_W,
                rel_bottom * _POS_W,
                aspect * _ASPECT_W,
                density * _DENSITY_W,
            ],
        ]
    ).astype(np.float32)
    norm = np.linalg.norm(vec)
    if norm > 0:
        vec = vec / norm
    return vec


class TemplateBank:
    """Feature vectors for every character in the charset at each font size."""

    def __init__(
        self,
        charset: str = DEFAULT_CHARSET,
        font_sizes: tuple[int, ...] = (32,),
    ) -> None:
        self.charset = charset
        self.font_sizes = tuple(font_sizes)
        self._chars: list[str] = []
        self._matrix: np.ndarray = np.zeros((0, 0), np.float32)
        self._build()

    def _build(self) -> None:
        vectors: list[np.ndarray] = []
        chars: list[str] = []
        for size in self.font_sizes:
            font = get_font(size)
            ascent, _descent, _cap_top = font_metrics(font)
            baseline = float(ascent)
            # Use the tallest glyph top as cap_top, since that is how the
            # pipeline measures a line. Otherwise mixed-case lines would be
            # measured differently from the templates and 'i' could pass for 'I'.
            tops = [font.getbbox(ch)[1] for ch in self.charset if ch.strip()]
            cap_top = float(min(tops)) if tops else float(_cap_top)
            for ch in self.charset:
                if ch == " ":
                    continue
                mask = render_glyph(ch, font)
                vec = extract_features(
                    mask, 0, mask.shape[1], baseline=baseline, cap_top=float(cap_top)
                )
                vectors.append(vec)
                chars.append(ch)
        self._matrix = np.vstack(vectors) if vectors else np.zeros((0, 1), np.float32)
        self._chars = chars

    @property
    def chars(self) -> list[str]:
        return list(dict.fromkeys(self._chars))

    def recognize(self, features: np.ndarray) -> tuple[str, float]:
        """Return ``(char, confidence)`` for a feature vector (cosine NN)."""
        if self._matrix.shape[0] == 0:
            return "?", 0.0
        sims = self._matrix @ features  # rows and features are unit vectors
        idx = int(np.argmax(sims))
        return self._chars[idx], float(sims[idx])

    def recognize_strip(
        self,
        line_mask: np.ndarray,
        x0: int,
        x1: int,
        baseline: float,
        cap_top: float,
    ) -> tuple[str, float]:
        """Classify the glyph in columns ``[x0, x1)`` of ``line_mask``."""
        feats = extract_features(line_mask, x0, x1, baseline, cap_top)
        return self.recognize(feats)
