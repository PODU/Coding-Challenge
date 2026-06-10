from __future__ import annotations

import math

import pytest

from llmwiki import HashingEmbedder


def test_determinism():
    e = HashingEmbedder(dim=64)
    a = e.embed(["the quick brown fox"])[0]
    b = e.embed(["the quick brown fox"])[0]
    assert a == b


def test_l2_normalized_and_dim():
    e = HashingEmbedder(dim=128)
    v = e.embed(["hello world"])[0]
    assert len(v) == 128
    assert math.isclose(math.sqrt(sum(x * x for x in v)), 1.0, rel_tol=1e-6)


def test_similar_texts_share_signal():
    e = HashingEmbedder(dim=256)
    shared = e.embed(["ada lovelace computing algorithm"])[0]
    related = e.embed(["ada lovelace algorithm pioneer"])[0]
    unrelated = e.embed(["quantum cryptography networking protocol"])[0]
    cos_rel = sum(a * b for a, b in zip(shared, related))
    cos_un = sum(a * b for a, b in zip(shared, unrelated))
    assert cos_rel > cos_un


def test_empty_text_is_zero_vector():
    e = HashingEmbedder(dim=32)
    v = e.embed([""])[0]
    assert all(x == 0.0 for x in v)
