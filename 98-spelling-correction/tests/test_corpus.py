"""Unit tests for corpus loading / normalization."""

from __future__ import annotations

from spellcorrect import corpus


def test_tokenize_lowercases_and_strips_punctuation_and_digits():
    text = "The QUICK, brown fox! 123 don't 42stop"
    assert corpus.tokenize(text) == [
        "the",
        "quick",
        "brown",
        "fox",
        "don",
        "t",
        "stop",
    ]


def test_normalize_strips_accents_digits_and_punctuation():
    assert corpus.normalize("Café!") == "cafe"
    assert corpus.normalize("HELLO-world") == "helloworld"
    assert corpus.normalize("123") == ""
    assert corpus.normalize("   ") == ""
    assert corpus.normalize("naïve") == "naive"


def test_count_text_sums_duplicate_words_and_total():
    table = corpus.count_text("the cat the dog the cat")
    assert table["the"] == 3
    assert table["cat"] == 2
    assert table["dog"] == 1
    assert sum(table.values()) == 6


def test_load_text_file(tmp_path):
    p = tmp_path / "corpus.txt"
    p.write_text("Apple apple APPLE banana.\nbanana!", encoding="utf-8")
    table = corpus.load_text_file(str(p))
    assert table["apple"] == 3
    assert table["banana"] == 2


def test_load_frequency_csv_skips_header_and_sums_duplicates(tmp_path):
    p = tmp_path / "freq.csv"
    p.write_text(
        "word,count\n"
        "the,100\n"
        "The,50\n"  # duplicate after normalization -> summed
        "quick,30\n"
        "bad-row\n"  # too few columns -> skipped
        "oops,notanumber\n"  # bad count -> skipped
        "123,5\n",  # word normalizes to empty -> skipped
        encoding="utf-8",
    )
    table = corpus.load_frequency_csv(str(p))
    assert table["the"] == 150
    assert table["quick"] == 30
    assert "oops" not in table
    assert "" not in table


def test_load_frequency_csv_no_header(tmp_path):
    p = tmp_path / "freq.csv"
    p.write_text("alpha,3\nbeta,7\n", encoding="utf-8")
    table = corpus.load_frequency_csv(str(p), has_header=False)
    assert table["alpha"] == 3
    assert table["beta"] == 7
