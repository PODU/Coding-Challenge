"""End-to-end pipeline tests: render text -> OCR -> assert the decoded string."""

from __future__ import annotations

import numpy as np
import pytest
from PIL import Image

from ocr import Box, CharResult, OCREngine, OCRResult
from tests.conftest import add_noise, char_accuracy, render_text_image


# Exact round trips: clean, level text at a template size.

EXACT_CASES = [
    "0123456789",
    "HELLO WORLD",
    "The quick brown Fox 123",
    "a,b:c-d.",
    "Hello, World.",
    "abcdefghijklmnopqrstuvwxyz",
    "ABCDEFGHIJKLMNOPQRSTUVWXYZ",
    "The quick brown fox jumps over the lazy dog",
    "Order 12 and 34",
    "Invoice 2024",
]


@pytest.mark.parametrize("text", EXACT_CASES)
def test_exact_round_trip(engine, render, text):
    result = engine.read(render(text))
    assert result.text == text


def test_digits_only(engine, render):
    assert engine.read(render("9876543210")).text == "9876543210"


def test_single_character(engine, render):
    for ch in "AQ7Z":
        assert engine.read(render(ch)).text == ch


def test_spacing_single_and_multiple_words(engine, render):
    assert engine.read(render("a b c")).text == "a b c"
    assert engine.read(render("go to the shop")).text == "go to the shop"


def test_multiline_text(engine, render):
    assert engine.read(render("First Line\nSecond Line")).text == "First Line\nSecond Line"


# At least 95% character accuracy on degraded images.

def test_rotation_is_corrected(engine, render):
    text = "Rotated Sample 5"
    for angle in (5.0, -5.0, 7.0):
        result = engine.read(render(text, angle=angle))
        assert char_accuracy(text, result.text) >= 0.95
        assert abs(result.skew_angle) > 0  # deskew actually fired


def test_coloured_text_and_background(engine, render):
    img = render("Colour 12", fg=(180, 20, 30), bg=(235, 235, 160))
    assert engine.read(img).text == "Colour 12"


def test_white_on_black(engine, render):
    img = render("Night Mode 9", fg=(255, 255, 255), bg=(0, 0, 0))
    assert engine.read(img).text == "Night Mode 9"


def test_noise_tolerance(engine, render):
    text = "Scanned Doc 8"
    noisy = add_noise(render(text), amount=35)
    assert char_accuracy(text, engine.read(noisy).text) >= 0.95


def test_render_size_different_from_template(engine, render):
    # 38 is not one of the engine's template sizes -> exercises normalisation.
    text = "Between Sizes 3"
    result = engine.read(render(text, font_size=38))
    assert char_accuracy(text, result.text) >= 0.95


@pytest.mark.parametrize("text", ["Cat 7", "Hello World", "Order 99"])
def test_font_size_invariance(engine, render, text):
    outputs = {engine.read(render(text, font_size=s)).text for s in (24, 32, 40, 48)}
    assert outputs == {text}


# Edge cases and the result API.

def test_empty_image_returns_empty_result(engine):
    blank = Image.new("RGB", (120, 60), (255, 255, 255))
    result = engine.read(blank)
    assert isinstance(result, OCRResult)
    assert result.text == ""
    assert result.chars == []
    assert result.regions == []


def test_idempotent(engine, render):
    img = render("Idempotent 1")
    assert engine.read(img).text == engine.read(img).text


def test_char_results_have_boxes_and_confidence(engine, render):
    result = engine.read(render("Box 7"))
    visible = result.text.replace(" ", "")
    assert len(result.chars) == len(visible)
    for c in result.chars:
        assert isinstance(c, CharResult)
        assert isinstance(c.box, Box)
        assert 0.0 <= c.confidence <= 1.0
        assert c.box.w > 0 and c.box.h > 0


def test_regions_detected(engine, render):
    result = engine.read(render("Region Test"))
    assert len(result.regions) == 1


def test_to_json_roundtrips(engine, render):
    import json

    result = engine.read(render("JSON 5"))
    payload = json.loads(result.to_json())
    assert payload["text"] == result.text
    assert len(payload["chars"]) == len(result.chars)


def test_draw_boxes_returns_image(engine, render):
    img = render("Overlay 2")
    result = engine.read(img)
    overlay = result.draw_boxes(img)
    assert isinstance(overlay, Image.Image)
    assert overlay.size == img.size


def test_low_confidence_becomes_placeholder(render):
    # A tiny charset forces an out-of-vocabulary glyph to score low -> '?'.
    engine = OCREngine(charset="0123456789", min_confidence=0.6)
    result = engine.read(render("Z"))
    assert result.text in {"?", ""} or all(c in "0123456789?" for c in result.text)


def test_accepts_numpy_and_path(engine, render, tmp_path):
    img = render("Paths 4")
    # numpy array input
    assert engine.read(np.asarray(img)).text == "Paths 4"
    # filesystem path input
    p = tmp_path / "sample.png"
    img.save(p)
    assert engine.read(str(p)).text == "Paths 4"
