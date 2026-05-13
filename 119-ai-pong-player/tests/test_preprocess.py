"""Tests for the pure preprocessing pipeline."""

from __future__ import annotations

import numpy as np

from pong_ai.preprocess import (
    CROP_TOP,
    OBS_DIM,
    RESIZE_DIM,
    frame_diff,
    preprocess_frame,
)


def _blank_frame() -> np.ndarray:
    return np.zeros((210, 160, 3), dtype=np.uint8)


def test_output_shape_dtype_and_range():
    frame = _blank_frame()
    frame[:] = 123  # uniform mid-gray
    out = preprocess_frame(frame)
    assert out.shape == (OBS_DIM,)
    assert out.dtype == np.float32
    assert out.min() >= 0.0 and out.max() <= 1.0
    np.testing.assert_allclose(out, 123.0 / 255.0, rtol=0, atol=1e-6)


def test_white_ball_lands_at_expected_downsampled_index():
    frame = _blank_frame()
    # Place a single white pixel inside the cropped playfield. Choose a row/col
    # that survives the ::2 subsample exactly.
    row, col = CROP_TOP + 40, 50  # even offset within crop -> kept by ::2
    frame[row, col] = 255
    out = preprocess_frame(frame).reshape(RESIZE_DIM, RESIZE_DIM)

    exp_r = (row - CROP_TOP) // 2
    exp_c = col // 2
    assert out[exp_r, exp_c] == 1.0
    # exactly one bright pixel
    assert np.count_nonzero(out >= 0.5) == 1


def test_grayscale_averages_channels():
    frame = _blank_frame()
    frame[CROP_TOP, 0] = (30, 60, 90)  # mean = 60
    out = preprocess_frame(frame).reshape(RESIZE_DIM, RESIZE_DIM)
    np.testing.assert_allclose(out[0, 0], 60.0 / 255.0, atol=1e-6)


def test_frame_diff_none_is_zeros():
    cur = np.ones(OBS_DIM, dtype=np.float32)
    diff = frame_diff(cur, None)
    assert diff.shape == (OBS_DIM,)
    assert np.all(diff == 0.0)


def test_frame_diff_is_subtraction():
    a = np.full(OBS_DIM, 0.7, dtype=np.float32)
    b = np.full(OBS_DIM, 0.2, dtype=np.float32)
    np.testing.assert_allclose(frame_diff(a, b), a - b, atol=1e-7)


def test_handles_grayscale_input():
    gray = np.full((210, 160), 200, dtype=np.uint8)
    out = preprocess_frame(gray)
    assert out.shape == (OBS_DIM,)
    np.testing.assert_allclose(out, 200.0 / 255.0, atol=1e-6)
