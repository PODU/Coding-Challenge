from __future__ import annotations

from carousel.layout import layout_slide
from carousel.models import Profile, Slide, Style


def measure_synthetic(text: str, size: float) -> float:
    """Deterministic measure for layout unit tests: ~0.5em per character."""
    return len(text) * size * 0.5


def test_wrapping_at_known_width():
    # content width = size - 2*pad. With size=1000, pad=80 -> width 840.
    # synthetic measure: 0.5em/char. At size=40 each char = 20px -> 42 chars/line.
    style = Style(size=1000, padding=80, font_size=40, min_font_size=40, page_numbers=False)
    body = " ".join(["word"] * 40)  # 40 words * (4 chars + space) ~ wraps
    layout = layout_slide(Slide(body_lines=[body]), style, measure_synthetic)
    assert len(layout.lines) > 1
    width = style.size - 2 * style.pad
    for line in layout.lines:
        assert measure_synthetic(line.text, line.size) <= width + 1e-6


def test_long_single_token_hard_breaks():
    style = Style(size=1000, padding=80, font_size=40, min_font_size=40, page_numbers=False)
    huge = "x" * 500
    layout = layout_slide(Slide(body_lines=[huge]), style, measure_synthetic)
    assert len(layout.lines) > 1
    width = style.size - 2 * style.pad
    for line in layout.lines:
        assert measure_synthetic(line.text, line.size) <= width + 1e-6


def test_auto_shrink_kicks_in():
    style = Style(size=600, padding=60, font_size=120, min_font_size=20, page_numbers=False)
    body = ("Lorem ipsum dolor sit amet " * 20).strip()
    layout = layout_slide(Slide(body_lines=[body]), style, measure_synthetic)
    assert layout.font_size < 120  # shrank to fit
    assert layout.font_size >= 20


def test_ellipsis_truncation_at_floor():
    style = Style(size=400, padding=40, font_size=30, min_font_size=30, page_numbers=False)
    body = ("alpha beta gamma delta epsilon " * 50).strip()
    layout = layout_slide(Slide(body_lines=[body]), style, measure_synthetic)
    assert layout.truncated is True
    assert layout.lines[-1].text.endswith("…")


def test_profile_box_top_left():
    style = Style(size=1080, page_numbers=False)
    profile = Profile(image="me.png", name="Jane", handle="@jane")
    layout = layout_slide(Slide(body_lines=["hi"]), style, measure_synthetic, profile=profile)
    assert layout.profile is not None
    assert layout.profile.x == style.pad
    assert layout.profile.y == style.pad
    # Body starts below the header region.
    assert layout.content_y > style.pad


def test_no_profile_no_header_reserved():
    style = Style(size=1080, page_numbers=False)
    layout = layout_slide(Slide(body_lines=["hi"]), style, measure_synthetic, profile=Profile())
    assert layout.profile is None
    assert layout.content_y == style.pad


def test_empty_slide_produces_no_lines():
    style = Style(size=1080, page_numbers=False)
    layout = layout_slide(Slide(), style, measure_synthetic)
    assert layout.lines == []
