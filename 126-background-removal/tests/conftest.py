"""Shared pytest fixtures: synthetic images for deterministic tests."""

from __future__ import annotations

from pathlib import Path

import numpy as np
import pytest
from PIL import Image, ImageDraw


def _portrait_rgb(size: int = 120) -> np.ndarray:
    """A gray background with a bright centred ellipse standing in for a subject."""
    img = Image.new("RGB", (size, size), (40, 40, 40))
    draw = ImageDraw.Draw(img)
    m = size // 6
    draw.ellipse([m, m, size - m, size - m], fill=(230, 200, 180))
    return np.asarray(img, dtype=np.uint8)


@pytest.fixture
def portrait_rgb() -> np.ndarray:
    return _portrait_rgb()


@pytest.fixture
def jpg_file(tmp_path: Path) -> Path:
    path = tmp_path / "subject.jpg"
    Image.fromarray(_portrait_rgb(), "RGB").save(path, "JPEG", quality=95)
    return path


@pytest.fixture
def png_file(tmp_path: Path) -> Path:
    path = tmp_path / "subject.png"
    Image.fromarray(_portrait_rgb(), "RGB").save(path, "PNG")
    return path


@pytest.fixture
def truncated_jpg(tmp_path: Path) -> Path:
    """A valid JPEG header followed by a cut-off body."""
    good = tmp_path / "full.jpg"
    Image.fromarray(_portrait_rgb(), "RGB").save(good, "JPEG", quality=95)
    data = good.read_bytes()
    bad = tmp_path / "truncated.jpg"
    bad.write_bytes(data[: len(data) // 2])
    good.unlink()
    return bad


@pytest.fixture
def not_an_image(tmp_path: Path) -> Path:
    path = tmp_path / "notes.jpg"
    path.write_text("this is plainly not image data")
    return path
