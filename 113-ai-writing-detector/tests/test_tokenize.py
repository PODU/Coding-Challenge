"""Tokeniser tests: deterministic counts and normalisation."""

from __future__ import annotations

import pytest

from aidetector.tokenize import (
    build_document,
    count_syllables,
    normalize,
    split_sentences,
    split_words,
)


def test_word_count_keeps_apostrophes() -> None:
    words = split_words("It's a quick, simple test — really.")
    assert words == ["It's", "a", "quick", "simple", "test", "really"]
    assert len(words) == 6


def test_sentence_split_on_terminators() -> None:
    sents = split_sentences("One. Two! Three? Four")
    assert sents == ["One", "Two", "Three", "Four"]


def test_sentence_split_ignores_trailing_quote() -> None:
    sents = split_sentences('He said "go." Then left.')
    assert len(sents) == 2


@pytest.mark.parametrize(
    "word,expected",
    [
        ("cat", 1),
        ("table", 2),
        ("beautiful", 3),
        ("the", 1),
        ("queue", 1),
        ("rhythm", 1),
        ("", 0),
    ],
)
def test_syllable_counts(word: str, expected: int) -> None:
    assert count_syllables(word) == expected


def test_normalize_smart_quotes_and_dashes() -> None:
    assert normalize("“hi” — it’s … fine") == '"hi" -- it\'s ... fine'


def test_document_basic_stats() -> None:
    doc = build_document("Hello world. This is a test.")
    assert doc.word_count == 6
    assert doc.sentence_count == 2
    assert doc.char_count == len("Hello world. This is a test.")
    assert doc.syllables == sum(
        count_syllables(w) for w in ["Hello", "world", "This", "is", "a", "test"]
    )


def test_empty_document() -> None:
    doc = build_document("   \n\t  ")
    assert doc.is_empty
    assert doc.word_count == 0
    assert doc.sentence_count == 0


def test_short_document_flag() -> None:
    assert build_document("Just three words.").is_short
    long_text = " ".join(["word"] * 30) + ". Another sentence here now."
    assert not build_document(long_text).is_short


def test_raw_preserved_for_punctuation() -> None:
    doc = build_document("A sentence — with an em dash.")
    assert "—" in doc.raw
    assert "--" in doc.text  # normalised form for matching
