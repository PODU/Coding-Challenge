"""Tests for grayscale conversion, Otsu thresholding and polarity detection."""

from __future__ import annotations

import numpy as np

from ocr.preprocess import binarize, otsu_threshold, to_gray
from tests.conftest import render_text_image


def test_to_gray_luminosity():
    rgb = np.zeros((4, 4, 3), dtype=np.uint8)
    rgb[..., 1] = 255  # pure green
    gray = to_gray(rgb)
    assert gray.shape == (4, 4)
    # Green has the largest luminosity weight (~0.587 * 255 ~= 150).
    assert 140 <= int(gray[0, 0]) <= 160


def test_otsu_threshold_bimodal():
    img = np.array([[60] * 20 + [200] * 20] * 30, dtype=np.uint8)
    t = otsu_threshold(img)
    assert 60 <= t < 200


def test_binarize_black_on_white_marks_ink_true():
    img = render_text_image("Ag", fg=(0, 0, 0), bg=(255, 255, 255))
    mask = binarize(np.asarray(img))
    # Ink (the dark text) must be the minority foreground marked True.
    assert mask.dtype == bool
    assert 0 < mask.mean() < 0.5
    # Borders are background -> False.
    assert not mask[0, :].any()


def test_binarize_polarity_autodetect_white_on_black():
    black_on_white = binarize(np.asarray(render_text_image("Ag", fg=(0, 0, 0), bg=(255, 255, 255))))
    white_on_black = binarize(np.asarray(render_text_image("Ag", fg=(255, 255, 255), bg=(0, 0, 0))))
    # Regardless of polarity, ink density should be similar and small.
    assert 0 < white_on_black.mean() < 0.5
    assert abs(black_on_white.mean() - white_on_black.mean()) < 0.05


def test_binarize_adaptive_runs():
    img = np.asarray(render_text_image("Hello", fg=(0, 0, 0), bg=(255, 255, 255)).convert("L"))
    mask = binarize(img, adaptive=True)
    assert mask.dtype == bool
    assert mask.any()
