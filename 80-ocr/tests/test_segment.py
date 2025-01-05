"""Tests for line / word / character segmentation."""

from __future__ import annotations

import numpy as np

from ocr.preprocess import binarize
from ocr.segment import (
    component_runs,
    segment,
    segment_chars_bounds,
    segment_lines_bounds,
)
from tests.conftest import render_text_image


def _mask(text: str, **kw) -> np.ndarray:
    return binarize(np.asarray(render_text_image(text, **kw)))


def test_three_chars_three_boxes():
    runs = component_runs(_mask("ABC"))
    assert len(runs) == 3


def test_word_count_two():
    spans = segment_chars_bounds(_mask("A B"))
    non_space = [s for s in spans if not s.is_space]
    spaces = [s for s in spans if s.is_space]
    assert len(non_space) == 2
    assert len(spaces) == 1  # one word gap


def test_touching_pair_rn_splits_into_two():
    # 'rn' is the classic touching pair; it must not be read as one blob.
    runs = component_runs(_mask("rn"))
    assert len(runs) == 2


def test_multiline_line_count():
    mask = _mask("First\nSecond\nThird")
    assert len(segment_lines_bounds(mask)) == 3


def test_single_char_one_box():
    assert len(component_runs(_mask("Q"))) == 1


def test_segment_returns_anchors_within_line():
    lines = segment(_mask("Mixed Case"))
    assert lines
    ln = lines[0]
    assert 0 <= ln.cap_top < ln.baseline <= (ln.y1 - ln.y0) + 2


def test_colon_is_single_character():
    # The two dots of ':' must merge into one character via component overlap.
    runs = component_runs(_mask("a:b"))
    assert len(runs) == 3
