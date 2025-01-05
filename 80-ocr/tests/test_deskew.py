"""Tests for skew estimation and correction."""

from __future__ import annotations

import numpy as np
import pytest

from ocr.deskew import deskew, estimate_skew
from ocr.preprocess import binarize
from tests.conftest import render_text_image


@pytest.mark.parametrize("angle", [7.0, -10.0, 4.0, -5.0])
def test_estimate_skew_recovers_angle(angle):
    img = render_text_image("Skew Estimation", angle=angle)
    mask = binarize(np.asarray(img))
    # The image is rotated by `angle`; the correcting rotation is about -angle.
    est = estimate_skew(mask)
    assert abs(est - (-angle)) <= 1.0


def test_deskew_increases_projection_variance():
    img = render_text_image("Horizontal Lines Please", angle=9.0)
    gray = np.asarray(img.convert("L"))
    mask = binarize(gray)

    def projection_variance(m: np.ndarray) -> float:
        return float(np.var(m.sum(axis=1)))

    before = projection_variance(mask)
    angle = estimate_skew(mask)
    corrected = binarize(deskew(gray, angle))
    after = projection_variance(corrected)
    assert after > before


def test_estimate_skew_zero_for_blank():
    assert estimate_skew(np.zeros((30, 30), dtype=bool)) == 0.0
