"""Shared pytest fixtures and PDF validation helpers (offline, deterministic)."""

from __future__ import annotations

import io
import os

import pypdf
import pytest

EXAMPLES = os.path.join(os.path.dirname(os.path.dirname(__file__)), "examples")


def assert_valid_pdf(data: bytes) -> None:
    """Assert ``data`` is a structurally valid, re-parseable PDF."""
    assert isinstance(data, (bytes, bytearray)), "PDF output must be bytes"
    assert data.startswith(b"%PDF-"), "PDF must start with %PDF- header"
    assert b"%%EOF" in data, "PDF must contain %%EOF trailer"
    reader = pypdf.PdfReader(io.BytesIO(data))
    assert len(reader.pages) >= 1, "PDF must have at least one page"


def pdf_page_count(data: bytes) -> int:
    return len(pypdf.PdfReader(io.BytesIO(data)).pages)


def pdf_text(data: bytes) -> str:
    reader = pypdf.PdfReader(io.BytesIO(data))
    return "\n".join(page.extract_text() or "" for page in reader.pages)


def pdf_media_box(data: bytes, page: int = 0) -> tuple[float, float]:
    box = pypdf.PdfReader(io.BytesIO(data)).pages[page].mediabox
    return float(box.width), float(box.height)


@pytest.fixture()
def examples_dir() -> str:
    return EXAMPLES


@pytest.fixture()
def sample_md() -> str:
    with open(os.path.join(EXAMPLES, "sample.md"), encoding="utf-8") as fh:
        return fh.read()


@pytest.fixture()
def long_md() -> str:
    with open(os.path.join(EXAMPLES, "long.md"), encoding="utf-8") as fh:
        return fh.read()


@pytest.fixture()
def in_examples(monkeypatch, examples_dir):
    """Run with cwd at examples/ so relative image paths resolve."""
    monkeypatch.chdir(examples_dir)
    yield examples_dir
