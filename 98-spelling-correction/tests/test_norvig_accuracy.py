"""Acceptance test against a sample of Norvig's spelling test set.

This is gated behind the ``acceptance`` marker and is **skipped by default**
because faithful accuracy (~75% on testset1) requires training on Norvig's
``big.txt`` (~6 MB), which is not bundled. Provide a training corpus via the
``SPELLCORRECT_BIG_TXT`` environment variable, or drop a ``big.txt`` into
``tests/data/`` to enable it::

    SPELLCORRECT_BIG_TXT=/path/to/big.txt python -m pytest -m acceptance

Run with::

    python -m pytest -m acceptance
"""

from __future__ import annotations

import os
import time
from pathlib import Path

import pytest

from spellcorrect import SpellCorrector

HERE = Path(__file__).parent
TESTSET = HERE / "data" / "spell-testset1-sample.txt"


def _find_big_txt() -> Path | None:
    env = os.environ.get("SPELLCORRECT_BIG_TXT")
    if env and Path(env).is_file():
        return Path(env)
    local = HERE / "data" / "big.txt"
    if local.is_file():
        return local
    return None


def _parse_testset(path: Path) -> list[tuple[str, str]]:
    """Parse Norvig's ``correct: wrong1 wrong2 ...`` format into pairs."""
    pairs: list[tuple[str, str]] = []
    for line in path.read_text(encoding="utf-8").splitlines():
        if ":" not in line:
            continue
        right, wrongs = line.split(":", 1)
        for wrong in wrongs.split():
            pairs.append((wrong.strip(), right.strip()))
    return pairs


@pytest.mark.acceptance
def test_norvig_testset_accuracy_and_throughput():
    big = _find_big_txt()
    if big is None:
        pytest.skip(
            "no big.txt available; set SPELLCORRECT_BIG_TXT or add tests/data/big.txt"
        )

    sc = SpellCorrector.from_text_file(str(big))
    pairs = _parse_testset(TESTSET)
    assert pairs, "test set parsed empty"

    start = time.perf_counter()
    good = sum(1 for wrong, right in pairs if sc.correct(wrong) == right)
    elapsed = time.perf_counter() - start

    accuracy = good / len(pairs)
    wps = len(pairs) / elapsed if elapsed else 0.0
    print(
        f"\nNorvig sample: {good}/{len(pairs)} = {accuracy:.1%} "
        f"({wps:.0f} words/sec)"
    )
    # Documented range is ~75% on the full testset1; the bundled sample is
    # easier, so we only assert a conservative floor here.
    assert accuracy >= 0.65


def test_testset_file_parses_without_training():
    """The bundled sample is well-formed even when big.txt is absent."""
    pairs = _parse_testset(TESTSET)
    assert len(pairs) > 20
    assert ("recieve", "receive") in pairs
