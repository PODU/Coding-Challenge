"""Segmentation and mask/edge tests (Steps 2, 3, 4)."""

from __future__ import annotations

import numpy as np

from ccbgremove.refine import build_output, mask_to_alpha
from ccbgremove.segment import (
    GrabCutSegmenter,
    OpaqueSegmenter,
    make_segmenter,
)


def test_opaque_backend_all_foreground(portrait_rgb):
    mask = OpaqueSegmenter().segment(portrait_rgb)
    assert mask.shape == portrait_rgb.shape[:2]
    assert np.all(mask == 1.0)


def test_grabcut_separates_subject_from_background(portrait_rgb):
    mask = GrabCutSegmenter().segment(portrait_rgb)
    h, w = mask.shape
    # Centre (subject) should be foreground; corners (background) should not.
    assert mask[h // 2, w // 2] == 1.0
    assert mask[1, 1] == 0.0
    assert mask[h - 2, w - 2] == 0.0
    # Foreground should be a sensible fraction, not the whole frame.
    assert 0.15 < mask.mean() < 0.9


def test_grabcut_tiny_image_degrades_gracefully():
    tiny = np.zeros((3, 3, 3), np.uint8)
    mask = GrabCutSegmenter().segment(tiny)
    assert np.all(mask == 1.0)


def test_make_segmenter_selection():
    assert make_segmenter("none").name == "none"
    assert make_segmenter("grabcut").name == "grabcut"
    # auto resolves to a concrete backend without raising.
    assert make_segmenter("auto").name in {"grabcut", "rembg"}


def test_apply_mask_sets_transparency(portrait_rgb):
    rgba = np.dstack(
        [portrait_rgb, np.full(portrait_rgb.shape[:2], 255, np.uint8)]
    )
    mask = GrabCutSegmenter().segment(portrait_rgb)
    out = build_output(rgba, mask, hard=True)
    h, w = mask.shape
    # Background transparent, subject opaque, RGB preserved.
    assert out[1, 1, 3] == 0
    assert out[h // 2, w // 2, 3] == 255
    assert np.array_equal(out[h // 2, w // 2, :3], rgba[h // 2, w // 2, :3])


def test_feathering_creates_partial_alpha():
    # Half foreground, half background -> a feathered edge must contain
    # intermediate alpha values.
    mask = np.zeros((40, 40), np.float32)
    mask[:, 20:] = 1.0
    alpha = mask_to_alpha(mask, feather=3.0)
    assert np.any((alpha > 0) & (alpha < 255))


def test_hard_edges_are_binary():
    mask = np.zeros((40, 40), np.float32)
    mask[:, 20:] = 1.0
    alpha = mask_to_alpha(mask, hard=True)
    assert set(np.unique(alpha)).issubset({0, 255})
