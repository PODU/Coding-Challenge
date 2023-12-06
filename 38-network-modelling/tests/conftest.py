"""Shared test helpers: locate fixture and sample-data CSV files."""

from __future__ import annotations

from pathlib import Path

import pytest

_ROOT = Path(__file__).resolve().parent.parent
FIXTURES = Path(__file__).resolve().parent / "fixtures"
DATA = _ROOT / "data"


@pytest.fixture
def fixtures_dir() -> Path:
    return FIXTURES


@pytest.fixture
def data_dir() -> Path:
    return DATA


def fixture(name: str) -> str:
    """Absolute path to a fixture CSV as a string."""
    return str(FIXTURES / name)
