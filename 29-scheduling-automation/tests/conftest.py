"""Shared test fixtures."""

from datetime import UTC, datetime
from pathlib import Path

import pytest

FIXTURES = Path(__file__).parent / "fixtures"


def utc(year, month, day, hour=0, minute=0):
    """Helper building a UTC-aware datetime."""
    return datetime(year, month, day, hour, minute, tzinfo=UTC)


@pytest.fixture
def fixtures_dir() -> Path:
    """Path to the ICS fixtures directory."""
    return FIXTURES


def read_fixture(name: str) -> str:
    """Read an ICS fixture file by name."""
    return (FIXTURES / name).read_text(encoding="utf-8")
