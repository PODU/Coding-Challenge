"""Load images from paths, bytes, PIL images or arrays."""

from __future__ import annotations

import io
import os
from typing import Union

import numpy as np
from PIL import Image, UnidentifiedImageError

from .types import UnreadableImageError

ImageInput = Union[str, os.PathLike, bytes, bytearray, "Image.Image", np.ndarray]


def to_pil(image: ImageInput) -> Image.Image:
    """Open a path, encoded bytes, a PIL image or a NumPy array as a PIL image."""
    try:
        if isinstance(image, Image.Image):
            return image
        if isinstance(image, np.ndarray):
            arr = image
            if arr.dtype != np.uint8:
                arr = np.clip(arr, 0, 255).astype(np.uint8)
            return Image.fromarray(arr)
        if isinstance(image, (bytes, bytearray)):
            return Image.open(io.BytesIO(bytes(image)))
        if isinstance(image, (str, os.PathLike)):
            return Image.open(os.fspath(image))
    except (UnidentifiedImageError, OSError, ValueError) as exc:
        raise UnreadableImageError(f"Could not decode image: {exc}") from exc
    raise UnreadableImageError(f"Unsupported image input type: {type(image)!r}")


def load_gray(image: ImageInput) -> np.ndarray:
    """Load *image* and return a 2-D ``uint8`` grayscale array."""
    pil = to_pil(image)
    if pil.mode != "L":
        pil = pil.convert("L")
    arr = np.asarray(pil, dtype=np.uint8)
    if arr.ndim != 2:
        raise UnreadableImageError("Expected a 2-D grayscale image after conversion.")
    return arr
