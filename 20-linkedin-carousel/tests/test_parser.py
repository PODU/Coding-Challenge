from __future__ import annotations

import json

import pytest

from carousel.models import Profile, Style, parse_color
from carousel.parser import (
    ParseError,
    load_config,
    parse_document,
    profile_from_config,
    style_from_config,
)


def test_separator_splits_slides():
    text = "# A\nbody a\n---\n# B\nbody b\n---\n# C\nbody c"
    slides = parse_document(text)
    assert len(slides) == 3
    assert [s.title for s in slides] == ["A", "B", "C"]


def test_title_extraction_and_body():
    slides = parse_document("# Hello World\nline one\nline two")
    assert len(slides) == 1
    assert slides[0].title == "Hello World"
    assert slides[0].body_lines == ["line one", "line two"]


def test_body_without_title():
    slides = parse_document("just some body text\nmore text")
    assert slides[0].title is None
    assert slides[0].body_lines == ["just some body text", "more text"]


def test_directive_bg_and_opacity():
    text = "<!-- bg: beach.jpg opacity: 0.3 -->\n# T\nbody"
    slide = parse_document(text)[0]
    assert slide.background_image == "beach.jpg"
    assert slide.background_opacity == pytest.approx(0.3)
    # Directive comment must not leak into the rendered body.
    assert all("bg:" not in line for line in slide.body_lines)


def test_directive_opacity_clamped():
    slide = parse_document("<!-- bg: x.png opacity: 5 -->\nbody")[0]
    assert slide.background_opacity == 1.0


def test_quoted_bg_path_with_spaces():
    slide = parse_document('<!-- bg: "my pic.png" opacity: 0.5 -->\nbody')[0]
    assert slide.background_image == "my pic.png"


def test_empty_document_raises():
    with pytest.raises(ParseError):
        parse_document("   \n\n  ")


def test_only_separators_yields_blank_slide():
    slides = parse_document("---\n---")
    assert len(slides) == 1
    assert slides[0].title is None
    assert slides[0].body_lines == []


def test_custom_separator():
    slides = parse_document("a\n===\nb", separator="===")
    assert len(slides) == 2


def test_config_merge_precedence(tmp_path):
    cfg = {
        "style": {"background": "#000000", "foreground": "#FFFFFF", "font_size": 40},
        "profile": {"name": "Jane", "handle": "@jane"},
    }
    path = tmp_path / "style.json"
    path.write_text(json.dumps(cfg), encoding="utf-8")
    data = load_config(str(path))

    style = style_from_config(data)
    profile = profile_from_config(data)
    assert style.background == (0, 0, 0)
    assert style.foreground == (255, 255, 255)
    assert style.font_size == 40
    assert profile.name == "Jane"
    assert profile.handle == "@jane"

    # Defaults remain for unspecified keys.
    default = Style()
    assert style.size == default.size


def test_missing_config_path_raises():
    with pytest.raises(ParseError):
        load_config("does-not-exist.json")


def test_load_config_none_returns_empty():
    assert load_config(None) == {}


def test_color_parsing_names_and_hex():
    assert parse_color("white") == (255, 255, 255)
    assert parse_color("#0A1F44") == (10, 31, 68)
    assert parse_color("#fff") == (255, 255, 255)
    assert parse_color((1, 2, 3)) == (1, 2, 3)


def test_color_parsing_invalid_raises():
    with pytest.raises(ValueError):
        parse_color("not-a-color")
    with pytest.raises(ValueError):
        parse_color("#GGGGGG")
