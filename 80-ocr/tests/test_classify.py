"""Tests for the template-matching classifier."""

from __future__ import annotations

import numpy as np
import pytest

from ocr.classify import TemplateBank, extract_features
from ocr.glyphs import DEFAULT_CHARSET, font_metrics, get_font, render_glyph


@pytest.fixture(scope="module")
def bank() -> TemplateBank:
    return TemplateBank(font_sizes=(24, 32, 40, 48, 56))


def _classify_rendered(bank: TemplateBank, ch: str, size: int):
    font = get_font(size)
    ascent, _descent, _cap = font_metrics(font)
    tops = [font.getbbox(c)[1] for c in DEFAULT_CHARSET if c.strip()]
    cap_top = float(min(tops))
    mask = render_glyph(ch, font)
    feats = extract_features(mask, 0, mask.shape[1], baseline=float(ascent), cap_top=cap_top)
    return bank.recognize(feats)


def test_every_template_recognises_itself(bank):
    # Each stored template's nearest neighbour must be its own label.
    mat, chars = bank._matrix, bank._chars
    for i, ch in enumerate(chars):
        pred = chars[int(np.argmax(mat @ mat[i]))]
        assert pred == ch


@pytest.mark.parametrize("size", [40, 50])
def test_full_charset_round_trip(bank, size):
    # Render every glyph and classify it back to itself (with confidence).
    errors = []
    for ch in DEFAULT_CHARSET:
        if not ch.strip():
            continue
        pred, conf = _classify_rendered(bank, ch, size)
        if pred != ch or conf < 0.4:
            errors.append((ch, pred, round(conf, 2)))
    assert errors == []


def test_chars_property_lists_charset(bank):
    expected = [c for c in DEFAULT_CHARSET if c != " "]
    assert set(bank.chars) == set(expected)


def test_blank_strip_low_confidence(bank):
    blank = np.zeros((40, 10), dtype=bool)
    feats = extract_features(blank, 0, 10, baseline=30.0, cap_top=6.0)
    _ch, conf = bank.recognize(feats)
    assert conf <= 0.5
