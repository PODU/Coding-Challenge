"""Tests for pronoun/word reflection."""

from __future__ import annotations

from eliza.reflect import reflect
from eliza.script import Script

TABLE = Script.load_default().reflections


def test_basic_first_to_second_person() -> None:
    assert reflect(["i", "am", "afraid", "of", "my", "mother"], TABLE) == [
        "you",
        "are",
        "afraid",
        "of",
        "your",
        "mother",
    ]


def test_unreflected_words_pass_through() -> None:
    assert reflect(["afraid"], TABLE) == ["afraid"]
    assert reflect(["mother", "and", "cake"], TABLE) == ["mother", "and", "cake"]


def test_case_insensitive_and_normalised() -> None:
    assert reflect(["My", "Father"], TABLE) == ["your", "father"]


def test_symmetric_pairs() -> None:
    assert reflect(["you", "are"], TABLE) == ["i", "am"]
    assert reflect(["your"], TABLE) == ["my"]
