"""Frame preprocessing: raw ``(210, 160, 3)`` Atari frame to a 6400-float vector.

The network is fed the difference of consecutive frames so it can see which
way the ball is moving.
"""

from __future__ import annotations

import numpy as np

# Rows 35..195 drop the score strip and bottom border, leaving 160x160, which
# halves cleanly to 80x80.
CROP_TOP = 35
CROP_BOTTOM = 195
RESIZE_DIM = 80
OBS_DIM = RESIZE_DIM * RESIZE_DIM  # 6400


def preprocess_frame(frame: np.ndarray) -> np.ndarray:
    """Crop, grayscale, downsample to 80x80 and flatten to floats in ``[0, 1]``."""
    arr = np.asarray(frame)
    if arr.ndim not in (2, 3):
        raise ValueError(f"expected a 2-D or 3-D frame, got shape {arr.shape}")

    cropped = arr[CROP_TOP:CROP_BOTTOM]
    if cropped.ndim == 3:
        gray = cropped[..., :3].astype(np.float32).mean(axis=2)
    else:
        gray = cropped.astype(np.float32)

    resized = _resize(gray, RESIZE_DIM)
    return (resized / 255.0).astype(np.float32).reshape(OBS_DIM)


def _resize(gray: np.ndarray, dim: int) -> np.ndarray:
    """Downsample to ``(dim, dim)``: striding for exact multiples, else nearest neighbour."""
    h, w = gray.shape
    if h % dim == 0 and w % dim == 0:
        return gray[:: h // dim, :: w // dim]
    rows = (np.arange(dim) * h // dim).clip(0, h - 1)
    cols = (np.arange(dim) * w // dim).clip(0, w - 1)
    return gray[np.ix_(rows, cols)]


def frame_diff(
    curr: np.ndarray, prev: np.ndarray | None
) -> np.ndarray:
    """Return ``curr - prev``, or zeros on the first frame (``prev is None``)."""
    curr = np.asarray(curr, dtype=np.float32)
    if prev is None:
        return np.zeros_like(curr)
    return (curr - np.asarray(prev, dtype=np.float32)).astype(np.float32)
