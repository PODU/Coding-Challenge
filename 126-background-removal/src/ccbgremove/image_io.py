"""Reading and writing images with Pillow.

Images are always loaded as ``(H, W, 4)`` uint8 RGBA arrays. Damaged files
raise :class:`InvalidImageError`.
"""

from __future__ import annotations

from pathlib import Path

import numpy as np
from PIL import Image, ImageFile, UnidentifiedImageError

# Make Pillow raise on a truncated image instead of returning a partial one.
ImageFile.LOAD_TRUNCATED_IMAGES = False

SUPPORTED_EXTS = {".jpg", ".jpeg", ".png"}


def is_supported(path: Path) -> bool:
    return path.suffix.lower() in SUPPORTED_EXTS


def load_rgba(path: Path) -> np.ndarray:
    """Load *path* as RGBA with every pixel fully opaque.

    Raises :class:`InvalidImageError` if it can't be decoded.
    """
    from .errors import InvalidImageError

    try:
        with Image.open(path) as im:
            im = im.convert("RGBA")
            # Decode fully now so a truncated file fails here.
            im.load()
            arr = np.asarray(im, dtype=np.uint8)
    except FileNotFoundError:
        raise InvalidImageError(f"{path}: file not found")
    except UnidentifiedImageError:
        raise InvalidImageError(f"{path}: not a recognised image format")
    except (OSError, ValueError) as exc:
        # Truncated files raise OSError; some decoder errors are ValueError.
        raise InvalidImageError(f"{path}: could not decode image ({exc})")

    if arr.ndim != 3 or arr.shape[2] != 4:
        raise InvalidImageError(f"{path}: unexpected image shape {arr.shape}")

    # Ignore any alpha the source had.
    arr = arr.copy()
    arr[:, :, 3] = 255
    return arr


def save_png(path: Path, rgba: np.ndarray) -> None:
    if rgba.ndim != 3 or rgba.shape[2] != 4:
        raise ValueError(f"expected RGBA array, got shape {rgba.shape}")
    path.parent.mkdir(parents=True, exist_ok=True)
    Image.fromarray(rgba, mode="RGBA").save(path, format="PNG")
