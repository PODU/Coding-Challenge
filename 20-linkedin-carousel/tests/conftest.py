"""Shared pytest fixtures and helpers.

All assets are generated on the fly (small PNGs via Pillow, short Markdown
docs) so the suite needs no committed binaries and runs fully offline.
"""

from __future__ import annotations

from pathlib import Path

import pytest
from PIL import Image
from pypdf import PdfReader


@pytest.fixture
def make_png():
    def _make(path: Path, color=(200, 60, 60), size=(256, 256)) -> Path:
        Image.new("RGB", size, color).save(path)
        return path

    return _make


@pytest.fixture
def page_count():
    def _count(pdf_path: Path) -> int:
        return len(PdfReader(str(pdf_path)).pages)

    return _count


@pytest.fixture
def is_pdf():
    def _is_pdf(pdf_path: Path) -> bool:
        return Path(pdf_path).read_bytes().startswith(b"%PDF-")

    return _is_pdf
