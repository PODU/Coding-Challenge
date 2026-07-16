"""Foreground segmentation backends.

Each backend has ``segment(rgb)``, taking an ``(H, W, 3)`` uint8 image and
returning an ``(H, W)`` float32 mask where 1 is subject and 0 is background.

* :class:`GrabCutSegmenter` uses OpenCV GrabCut and needs no downloads.
* :class:`RembgSegmenter` uses the optional ``rembg`` package (U^2-Net). It is
  picked automatically when installed and does better on hair.
* :class:`OpaqueSegmenter` treats everything as foreground.
"""

from __future__ import annotations

from typing import Protocol

import cv2
import numpy as np


class Segmenter(Protocol):
    name: str

    def segment(self, rgb: np.ndarray) -> np.ndarray:
        """Return an (H, W) float32 foreground mask in [0, 1]."""
        ...


class OpaqueSegmenter:

    name = "none"

    def segment(self, rgb: np.ndarray) -> np.ndarray:
        h, w = rgb.shape[:2]
        return np.ones((h, w), dtype=np.float32)


class GrabCutSegmenter:
    """OpenCV GrabCut, seeded with a rectangle inset *margin* from each edge.

    This assumes the subject is roughly centred, as in most portraits.
    """

    name = "grabcut"

    def __init__(self, margin: float = 0.08, iterations: int = 5) -> None:
        if not 0.0 <= margin < 0.5:
            raise ValueError("margin must be in [0, 0.5)")
        self.margin = margin
        self.iterations = max(1, iterations)

    def segment(self, rgb: np.ndarray) -> np.ndarray:
        h, w = rgb.shape[:2]
        mx = int(w * self.margin)
        my = int(h * self.margin)
        rect_w = w - 2 * mx
        rect_h = h - 2 * my

        # GrabCut needs some background outside the rectangle to learn from.
        # If the image is too small for that, keep everything.
        if mx < 1 or my < 1 or rect_w < 2 or rect_h < 2:
            return np.ones((h, w), dtype=np.float32)

        bgr = cv2.cvtColor(rgb, cv2.COLOR_RGB2BGR)
        mask = np.zeros((h, w), np.uint8)
        bgd_model = np.zeros((1, 65), np.float64)
        fgd_model = np.zeros((1, 65), np.float64)
        rect = (mx, my, rect_w, rect_h)

        cv2.grabCut(
            bgr, mask, rect, bgd_model, fgd_model,
            self.iterations, cv2.GC_INIT_WITH_RECT,
        )

        return np.where(
            (mask == cv2.GC_FGD) | (mask == cv2.GC_PR_FGD), 1.0, 0.0
        ).astype(np.float32)


class RembgSegmenter:
    """Segmentation with ``rembg`` (U^2-Net). Optional dependency."""

    name = "rembg"

    def __init__(self, model: str = "u2net") -> None:
        try:
            from rembg import new_session  # noqa: WPS433 (optional import)
        except ImportError as exc:  # pragma: no cover - depends on environment
            raise RuntimeError(
                "rembg is not installed; install with "
                "'pip install ccbgremove[neural]' or choose --backend grabcut"
            ) from exc
        self._session = new_session(model)

    def segment(self, rgb: np.ndarray) -> np.ndarray:  # pragma: no cover
        from rembg import remove

        mask = remove(rgb, session=self._session, only_mask=True)
        if mask.ndim == 3:
            mask = mask[:, :, 0]
        return mask.astype(np.float32) / 255.0


def rembg_available() -> bool:
    try:
        import rembg  # noqa: F401
    except ImportError:
        return False
    return True


def make_segmenter(name: str = "auto", **kwargs) -> Segmenter:
    """Create a backend by name. ``auto`` means rembg if installed, else GrabCut."""
    name = name.lower()
    if name == "auto":
        name = "rembg" if rembg_available() else "grabcut"

    if name == "none":
        return OpaqueSegmenter()
    if name == "grabcut":
        gc_kwargs = {k: kwargs[k] for k in ("margin", "iterations") if k in kwargs}
        return GrabCutSegmenter(**gc_kwargs)
    if name == "rembg":
        return RembgSegmenter(model=kwargs.get("model", "u2net"))
    raise ValueError(f"unknown backend '{name}'")
