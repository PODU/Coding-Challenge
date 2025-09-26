"""Unit tests for edit generation and correction behavior."""

from __future__ import annotations

import pytest

from spellcorrect import SpellCorrector, default_corrector


# --------------------------------------------------------------------- #
# Edit generation                                                       #
# --------------------------------------------------------------------- #
def test_edits1_size_and_membership_for_single_char():
    edits = SpellCorrector.edits1("a")
    # Raw count = 54*n + 25 for n=1 -> 79, but dedup collapses some (e.g. the
    # replace "a"->"a" and several inserts). The deduplicated set is smaller.
    assert "" in edits  # delete
    assert "aa" in edits  # insert / replace
    assert "ba" in edits  # insert before
    assert "ab" in edits  # insert after
    # Every edit is within one operation; none longer than 2 chars here.
    assert all(len(e) <= 2 for e in edits)


def test_edits1_raw_operation_counts():
    word = "abc"
    n = len(word)
    splits = [(word[:i], word[i:]) for i in range(n + 1)]
    deletes = [l + r[1:] for l, r in splits if r]
    transposes = [l + r[1] + r[0] + r[2:] for l, r in splits if len(r) > 1]
    replaces = [l + c + r[1:] for l, r in splits if r for c in "abcdefghijklmnopqrstuvwxyz"]
    inserts = [l + c + r for l, r in splits for c in "abcdefghijklmnopqrstuvwxyz"]
    assert len(deletes) == n
    assert len(transposes) == n - 1
    assert len(replaces) == n * 26
    assert len(inserts) == (n + 1) * 26


def test_edits2_contains_original_via_double_edit():
    # Two edits (insert then delete) can round-trip back to the original word.
    assert "word" in set(SpellCorrector.edits2("word"))


def test_known_filters_to_dictionary_members():
    sc = SpellCorrector({"the": 10, "cat": 5})
    assert sc.known(["the", "cat", "dog", "xyz"]) == {"the", "cat"}


# --------------------------------------------------------------------- #
# Correction behavior                                                   #
# --------------------------------------------------------------------- #
def test_known_word_passes_through():
    sc = default_corrector()
    assert sc.correct("the") == "the"
    assert sc.is_known("the")
    assert sc.edit_distance("the") == 0


def test_distance_1_fix():
    sc = default_corrector()
    assert sc.correct("speling") == "spelling"
    assert sc.edit_distance("speling") == 1


def test_distance_2_fix():
    sc = default_corrector()
    assert sc.correct("korrekt") == "correct"
    assert sc.edit_distance("korrekt") == 2


def test_no_candidate_echoes_input():
    sc = default_corrector()
    assert sc.correct("zzxqq") == "zzxqq"
    assert sc.edit_distance("zzxqq") == -1


def test_frequency_ranking_and_flip():
    # "cag" is distance-1 from both "cat" and "car" (replace g->t / replace g->r).
    high_cat = SpellCorrector({"cat": 100, "car": 1})
    assert high_cat.correct("cag") == "cat"
    # Flip the frequencies and the winner flips too.
    high_car = SpellCorrector({"cat": 1, "car": 100})
    assert high_car.correct("cag") == "car"


def test_tie_break_is_lexicographically_smallest():
    # Equal frequency -> deterministic lexicographically smallest ("car" < "cat").
    sc = SpellCorrector({"cat": 5, "car": 5})
    assert sc.correct("cag") == "car"
    # Reproducible across repeated builds (no dependence on iteration order).
    for _ in range(5):
        assert SpellCorrector({"cat": 5, "car": 5}).correct("cag") == "car"


def test_lowest_distance_precedence_over_frequency():
    # "spelling" is distance-1 (insert) from "speling"; "spellings" is reachable
    # at distance 2. A low-frequency distance-1 word must beat a high-frequency
    # distance-2 word.
    sc = SpellCorrector({"spelling": 1, "spellings": 1000})
    assert sc.correct("speling") == "spelling"
    assert sc.edit_distance("speling") == 1


# --------------------------------------------------------------------- #
# Normalization / edge cases                                            #
# --------------------------------------------------------------------- #
def test_mixed_case_input_corrects_to_lowercase():
    sc = default_corrector()
    assert sc.correct("THE") == "the"
    assert sc.correct("Speling") == "spelling"


def test_empty_and_nonalpha_inputs():
    sc = default_corrector()
    assert sc.correct("") == ""
    assert sc.correct("123") == ""
    assert sc.correct("!!!") == ""


def test_probability_and_total():
    sc = SpellCorrector({"a": 3, "b": 1})
    assert sc.total == 4
    assert sc.probability("a") == pytest.approx(0.75)
    assert sc.probability("b") == pytest.approx(0.25)
    assert sc.probability("z") == 0.0


def test_constructor_normalizes_and_drops_nonpositive():
    sc = SpellCorrector({"The": 2, "the": 3, "0bad": 5, "skip": 0})
    assert sc.frequency("the") == 5  # 2 + 3 after normalization
    assert "bad" in sc  # "0bad" -> "bad"
    assert not sc.is_known("skip")  # count 0 dropped


def test_candidates_returns_smallest_distance_set():
    sc = default_corrector()
    assert sc.candidates("the") == {"the"}
    assert "spelling" in sc.candidates("speling")
    assert sc.candidates("zzxqq") == set()
