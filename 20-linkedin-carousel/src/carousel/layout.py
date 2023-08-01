"""Slide layout: wrapping, shrinking text to fit, and placing the profile.

Coordinates here run top-down from the top-left corner. reportlab's y axis
points up, so the renderer flips them.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Callable, Optional

from .models import Profile, Slide, Style

# (text, font_size) -> width in px
Measure = Callable[[str, float], float]

ELLIPSIS = "…"


@dataclass
class TextLine:
    text: str
    x: float
    baseline: float  # top-down y of the text baseline, in px
    size: float
    is_title: bool = False


@dataclass
class ProfileBox:
    x: float
    y: float
    diameter: float
    name: Optional[str]
    handle: Optional[str]
    name_x: float
    name_baseline: float
    name_size: float
    handle_baseline: float
    handle_size: float
    has_image: bool


@dataclass
class SlideLayout:
    lines: list[TextLine] = field(default_factory=list)
    profile: Optional[ProfileBox] = None
    font_size: float = 0.0
    truncated: bool = False
    content_x: float = 0.0
    content_y: float = 0.0
    content_width: float = 0.0
    content_height: float = 0.0


def _wrap_paragraph(text: str, size: float, width: float, measure: Measure) -> list[str]:
    """Wrap one paragraph to ``width``, hard-breaking over-long tokens."""
    if text.strip() == "":
        return [""]
    words = text.split()
    lines: list[str] = []
    current = ""
    for word in words:
        trial = word if not current else current + " " + word
        if measure(trial, size) <= width:
            current = trial
            continue
        if current:
            lines.append(current)
            current = ""
        if measure(word, size) <= width:
            current = word
            continue
        # A single word wider than the box gets broken mid-word.
        piece = ""
        for ch in word:
            if measure(piece + ch, size) <= width or not piece:
                piece += ch
            else:
                lines.append(piece)
                piece = ch
        current = piece
    if current:
        lines.append(current)
    return lines or [""]


def _build_lines(
    slide: Slide, size: float, width: float, measure: Measure
) -> list[tuple[str, float, bool]]:
    """Wrap the title and body at the given body size, as (text, size, is_title)."""
    out: list[tuple[str, float, bool]] = []
    if slide.title:
        title_size = round(size * 1.4)
        for ln in _wrap_paragraph(slide.title, title_size, width, measure):
            out.append((ln, title_size, True))
        out.append(("", round(size * 0.5), False))  # gap after title
    for para in slide.body_lines or []:
        for ln in _wrap_paragraph(para, size, width, measure):
            out.append((ln, size, False))
    return out


def _line_height(size: float, spacing: float) -> float:
    return size * spacing


def _total_height(
    lines: list[tuple[str, float, bool]], spacing: float
) -> float:
    return sum(_line_height(s, spacing) for _, s, _ in lines)


def layout_slide(
    slide: Slide,
    style: Style,
    measure: Measure,
    profile: Optional[Profile] = None,
) -> SlideLayout:
    """Lay out one slide, shrinking the text until it fits.

    If it still doesn't fit at the minimum size, the overflow is cut off and
    the last line ends in an ellipsis.
    """
    size = style.size
    pad = style.pad
    spacing = style.line_spacing

    content_x = pad
    content_w = size - 2 * pad

    profile_box: Optional[ProfileBox] = None
    header_h = 0.0
    if profile is not None and profile.has_content:
        diameter = round(size * 0.085)
        name_size = round(diameter * 0.42)
        handle_size = round(diameter * 0.34)
        has_image = bool(profile.image)
        text_x = pad + (diameter + round(size * 0.02) if has_image else 0)
        profile_box = ProfileBox(
            x=pad,
            y=pad,
            diameter=diameter,
            name=profile.name,
            handle=profile.handle,
            name_x=text_x,
            name_baseline=pad + diameter * 0.42,
            name_size=name_size,
            handle_baseline=pad + diameter * 0.82,
            handle_size=handle_size,
            has_image=has_image,
        )
        header_h = diameter + round(size * 0.05)

    content_top = pad + header_h
    page_reserve = round(size * 0.05) if style.page_numbers else 0
    content_bottom = size - pad - page_reserve
    avail_h = max(0.0, content_bottom - content_top)

    floor = style.min_font_size
    truncated = False
    for chosen in range(style.font_size, floor - 1, -1):
        lines = _build_lines(slide, chosen, content_w, measure)
        if _total_height(lines, spacing) <= avail_h:
            break
    else:
        chosen = floor
        lines = _build_lines(slide, floor, content_w, measure)
        lines, truncated = _truncate_to_fit(lines, avail_h, content_w, spacing, measure)

    out_lines: list[TextLine] = []
    y = content_top
    for text, line_size, is_title in lines:
        baseline = y + line_size  # about one em below the top of the line
        if text != "":
            out_lines.append(
                TextLine(text=text, x=content_x, baseline=baseline, size=line_size, is_title=is_title)
            )
        y += _line_height(line_size, spacing)

    return SlideLayout(
        lines=out_lines,
        profile=profile_box,
        font_size=chosen,
        truncated=truncated,
        content_x=content_x,
        content_y=content_top,
        content_width=content_w,
        content_height=avail_h,
    )


def _truncate_to_fit(
    lines: list[tuple[str, float, bool]],
    avail_h: float,
    width: float,
    spacing: float,
    measure: Measure,
) -> tuple[list[tuple[str, float, bool]], bool]:
    """Keep the lines that fit and end the last one with an ellipsis."""
    kept: list[tuple[str, float, bool]] = []
    used = 0.0
    for item in lines:
        _, line_size, _ = item
        lh = _line_height(line_size, spacing)
        if used + lh > avail_h and kept:
            break
        kept.append(item)
        used += lh
    truncated = len(kept) < len(lines)
    if truncated and kept:
        text, line_size, is_title = kept[-1]
        kept[-1] = (_ellipsize(text, line_size, width, measure), line_size, is_title)
    return kept, truncated


def _ellipsize(text: str, size: float, width: float, measure: Measure) -> str:
    candidate = text.rstrip()
    if measure(candidate + ELLIPSIS, size) <= width:
        return candidate + ELLIPSIS
    while candidate and measure(candidate + ELLIPSIS, size) > width:
        candidate = candidate[:-1].rstrip()
    return (candidate + ELLIPSIS) if candidate else ELLIPSIS
