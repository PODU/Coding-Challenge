"""Grayscale conversion and binarisation into an ink mask."""

from __future__ import annotations

import numpy as np


def to_gray(arr: np.ndarray) -> np.ndarray:
    """Convert an RGB(A) or grayscale array to grayscale (Rec. 601 weights)."""
    if arr.ndim == 2:
        return arr.astype(np.uint8)
    if arr.ndim == 3:
        rgb = arr[:, :, :3].astype(np.float64)
        lum = rgb[:, :, 0] * 0.299 + rgb[:, :, 1] * 0.587 + rgb[:, :, 2] * 0.114
        return np.clip(lum, 0, 255).astype(np.uint8)
    raise ValueError(f"Unsupported array shape for to_gray: {arr.shape}")


def otsu_threshold(gray: np.ndarray) -> int:
    """Otsu's global threshold."""
    hist, _ = np.histogram(gray.ravel(), bins=256, range=(0, 256))
    total = gray.size
    if total == 0:
        return 127
    hist = hist.astype(np.float64)
    omega = np.cumsum(hist)
    mu = np.cumsum(hist * np.arange(256))
    mu_t = mu[-1]
    denom = omega * (total - omega)
    with np.errstate(divide="ignore", invalid="ignore"):
        sigma_b = (mu_t * omega - mu * total) ** 2 / denom
    sigma_b[~np.isfinite(sigma_b)] = -1.0
    return int(np.argmax(sigma_b))


def binarize(
    gray: np.ndarray,
    *,
    threshold: int | None = None,
    adaptive: bool = False,
    block: int = 31,
    offset: int = 10,
) -> np.ndarray:
    """Return a boolean mask that is True for ink.

    Dark-on-light and light-on-dark text both work. ``adaptive=True`` uses a
    local-mean threshold, which copes better with uneven lighting.
    """
    gray = to_gray(gray)
    if adaptive:
        mask = _adaptive_mean(gray, block=block, offset=offset)
    else:
        t = otsu_threshold(gray) if threshold is None else threshold
        mask = gray <= t
    return _normalise_polarity(mask)


def _normalise_polarity(mask: np.ndarray) -> np.ndarray:
    """Invert the mask if True turned out to be the background.

    The border is assumed to be mostly background.
    """
    h, w = mask.shape
    if h == 0 or w == 0:
        return mask
    border = np.concatenate(
        [mask[0, :], mask[-1, :], mask[:, 0], mask[:, -1]]
    )
    return ~mask if border.mean() > 0.5 else mask


def _adaptive_mean(gray: np.ndarray, *, block: int, offset: int) -> np.ndarray:
    """Threshold each pixel against the mean of the block around it."""
    block = max(3, block | 1)  # must be odd
    pad = block // 2
    g = gray.astype(np.float64)
    padded = np.pad(g, pad, mode="edge")
    # Use an integral image so each box sum is four lookups.
    integral = padded.cumsum(axis=0).cumsum(axis=1)
    integral = np.pad(integral, ((1, 0), (1, 0)), mode="constant")
    h, w = gray.shape
    y0 = np.arange(h)
    x0 = np.arange(w)
    yy0, xx0 = np.meshgrid(y0, x0, indexing="ij")
    yy1 = yy0 + block
    xx1 = xx0 + block
    area = block * block
    total = (
        integral[yy1, xx1]
        - integral[yy0, xx1]
        - integral[yy1, xx0]
        + integral[yy0, xx0]
    )
    local_mean = total / area
    return g <= (local_mean - offset)
