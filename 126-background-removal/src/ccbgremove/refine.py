"""Turn a foreground mask into the alpha channel of the output.

By default the mask edge is blurred a little so the boundary is partly
transparent rather than jagged, which matters most around hair.
"""

from __future__ import annotations

import cv2
import numpy as np


def mask_to_alpha(
    mask: np.ndarray, feather: float = 2.0, hard: bool = False
) -> np.ndarray:
    """Convert a [0, 1] float mask to uint8 alpha.

    With *hard* set, or *feather* at 0, the result is strictly 0 or 255.
    """
    m = np.clip(mask.astype(np.float32), 0.0, 1.0)

    if hard or feather <= 0:
        return (m >= 0.5).astype(np.uint8) * 255

    radius = max(1, int(round(feather)))
    ksize = radius * 2 + 1
    soft = cv2.GaussianBlur(m, (ksize, ksize), 0)
    soft = np.clip(soft, 0.0, 1.0)
    return (soft * 255.0 + 0.5).astype(np.uint8)


def apply_alpha(rgba: np.ndarray, alpha: np.ndarray) -> np.ndarray:
    """Return a copy of *rgba* with its alpha channel replaced."""
    out = rgba.copy()
    out[:, :, 3] = alpha
    return out


def build_output(
    rgba: np.ndarray,
    mask: np.ndarray,
    feather: float = 2.0,
    hard: bool = False,
) -> np.ndarray:
    alpha = mask_to_alpha(mask, feather=feather, hard=hard)
    return apply_alpha(rgba, alpha)
