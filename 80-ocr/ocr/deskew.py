"""Estimate and correct small text rotation.

The mask is rotated through a range of angles, keeping the one where the row
sums vary the most: level text gives sharp, well-separated rows.
"""

from __future__ import annotations

import numpy as np
from PIL import Image


def _rotate_mask(mask: np.ndarray, angle: float) -> np.ndarray:
    """Rotate a boolean mask by *angle* degrees anticlockwise, keeping its size."""
    img = Image.fromarray(mask.astype(np.uint8) * 255)
    rot = img.rotate(angle, resample=Image.BILINEAR, expand=False, fillcolor=0)
    return np.asarray(rot) > 127


def _projection_score(mask: np.ndarray) -> float:
    """Variance of the row sums. Higher means sharper text rows."""
    return float(np.var(mask.sum(axis=1).astype(np.float64)))


def estimate_skew(
    mask: np.ndarray,
    *,
    limit: float = 15.0,
    coarse_step: float = 1.0,
    fine_step: float = 0.2,
) -> float:
    """Return the anticlockwise rotation, in degrees, that levels the text.

    A coarse sweep finds the rough angle, then a finer sweep around it refines it.
    """
    if not mask.any():
        return 0.0

    def best_in(angles: np.ndarray) -> tuple[float, float]:
        best_a, best_s = 0.0, -1.0
        for a in angles:
            score = _projection_score(_rotate_mask(mask, float(a)))
            if score > best_s:
                best_s, best_a = score, float(a)
        return best_a, best_s

    coarse = np.arange(-limit, limit + coarse_step, coarse_step)
    best_a, _ = best_in(coarse)
    fine = np.arange(best_a - coarse_step, best_a + coarse_step + fine_step, fine_step)
    best_a, _ = best_in(fine)
    return round(best_a, 2)


def deskew(
    gray: np.ndarray, angle: float, *, fill: int | None = None
) -> np.ndarray:
    """Rotate a grayscale image by *angle* degrees anticlockwise."""
    if abs(angle) < 1e-3:
        return gray
    if fill is None:
        # Fill the new corners with the background colour, taken from the border.
        border = np.concatenate(
            [gray[0, :], gray[-1, :], gray[:, 0], gray[:, -1]]
        )
        fill = int(np.median(border))
    img = Image.fromarray(gray.astype(np.uint8))
    rot = img.rotate(angle, resample=Image.BICUBIC, expand=True, fillcolor=fill)
    return np.asarray(rot, dtype=np.uint8)
