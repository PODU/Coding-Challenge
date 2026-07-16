"""Decode/validate/encode tests (Steps 1 & 6)."""

from __future__ import annotations

from pathlib import Path

import numpy as np
import pytest
from PIL import Image

from ccbgremove import image_io
from ccbgremove.errors import InvalidImageError


def test_load_yields_rgba_full_opacity(jpg_file: Path):
    arr = image_io.load_rgba(jpg_file)
    assert arr.ndim == 3 and arr.shape[2] == 4
    assert arr.dtype == np.uint8
    # Every pixel fully opaque.
    assert np.all(arr[:, :, 3] == 255)


def test_load_png_matches_dimensions(png_file: Path):
    arr = image_io.load_rgba(png_file)
    with Image.open(png_file) as im:
        assert arr.shape[:2] == (im.height, im.width)


def test_save_png_roundtrip_dimensions(tmp_path: Path, portrait_rgb):
    rgba = np.dstack([portrait_rgb, np.full(portrait_rgb.shape[:2], 255, np.uint8)])
    out = tmp_path / "out.png"
    image_io.save_png(out, rgba)
    with Image.open(out) as im:
        assert im.mode == "RGBA"
        assert (im.height, im.width) == portrait_rgb.shape[:2]


def test_truncated_jpeg_rejected(truncated_jpg: Path):
    with pytest.raises(InvalidImageError):
        image_io.load_rgba(truncated_jpg)


def test_non_image_rejected(not_an_image: Path):
    with pytest.raises(InvalidImageError):
        image_io.load_rgba(not_an_image)


def test_missing_file_rejected(tmp_path: Path):
    with pytest.raises(InvalidImageError):
        image_io.load_rgba(tmp_path / "nope.png")


def test_is_supported():
    assert image_io.is_supported(Path("a.JPG"))
    assert image_io.is_supported(Path("a.jpeg"))
    assert image_io.is_supported(Path("a.png"))
    assert not image_io.is_supported(Path("a.gif"))
    assert not image_io.is_supported(Path("a.txt"))
