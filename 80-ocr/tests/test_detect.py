"""Tests for text detection: region bounding boxes and connected components."""

from __future__ import annotations

import numpy as np
from PIL import Image, ImageDraw

from ocr.detect import connected_components, find_text_regions, has_text
from ocr.glyphs import get_font
from ocr.preprocess import binarize
from ocr.types import Box


def _place(text: str, pos: tuple[int, int], size=(320, 140)) -> np.ndarray:
    canvas = Image.new("RGB", size, (255, 255, 255))
    draw = ImageDraw.Draw(canvas)
    draw.text(pos, text, font=get_font(40), fill=(0, 0, 0))
    return binarize(np.asarray(canvas.convert("L")))


def _ink_box(mask: np.ndarray) -> Box:
    ys, xs = np.nonzero(mask)
    return Box(int(xs.min()), int(ys.min()), int(xs.max() - xs.min() + 1), int(ys.max() - ys.min() + 1))


def test_find_regions_tight_box_at_various_positions():
    for pos in [(5, 5), (70, 40), (150, 12)]:
        mask = _place("Hi", pos)
        regions = find_text_regions(mask)
        assert len(regions) == 1
        assert regions[0].iou(_ink_box(mask)) >= 0.9


def test_find_regions_one_box_per_line():
    canvas = Image.new("RGB", (220, 170), (255, 255, 255))
    draw = ImageDraw.Draw(canvas)
    draw.text((10, 10), "One", font=get_font(40), fill=(0, 0, 0))
    draw.text((10, 90), "Two", font=get_font(40), fill=(0, 0, 0))
    mask = binarize(np.asarray(canvas.convert("L")))
    assert len(find_text_regions(mask)) == 2


def test_empty_image_has_no_regions():
    mask = np.zeros((40, 60), dtype=bool)
    assert find_text_regions(mask) == []
    assert not has_text(mask)


def test_connected_components_counts_blobs():
    mask = np.zeros((20, 20), dtype=bool)
    mask[2:6, 2:6] = True
    mask[10:14, 12:16] = True
    _labels, n = connected_components(mask)
    assert n == 2
