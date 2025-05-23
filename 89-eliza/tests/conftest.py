"""Shared fixtures for the ELIZA test-suite."""

from __future__ import annotations

import pytest

from eliza.engine import ElizaEngine
from eliza.script import Script


@pytest.fixture()
def script() -> Script:
    """The bundled classic DOCTOR script."""
    return Script.load_default()


@pytest.fixture()
def engine(script: Script) -> ElizaEngine:
    """A fresh, deterministic engine (cursors at 0) per test."""
    return ElizaEngine(script, reassembly="cycle")
