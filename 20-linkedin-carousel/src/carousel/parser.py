"""Parsing the input text and the JSON style file.

Slides are separated by ``---`` lines. A ``# Heading`` at the top of a slide
becomes its title. A slide can set a background with an HTML comment::

    <!-- bg: beach.jpg opacity: 0.3 -->
"""

from __future__ import annotations

import json
import re
from pathlib import Path
from typing import Any, Optional

from .models import Carousel, Profile, Slide, Style

DEFAULT_SEPARATOR = "---"

_DIRECTIVE_RE = re.compile(r"<!--(.*?)-->", re.DOTALL)
_BG_RE = re.compile(r"\bbg\s*:\s*(?P<v>\"[^\"]*\"|'[^']*'|\S+)", re.IGNORECASE)
_OPACITY_RE = re.compile(r"\bopacity\s*:\s*(?P<v>[0-9]*\.?[0-9]+)", re.IGNORECASE)


class ParseError(ValueError):
    """The input or config file can't be used."""


def _strip_quotes(value: str) -> str:
    if len(value) >= 2 and value[0] in "\"'" and value[-1] == value[0]:
        return value[1:-1]
    return value


def _extract_directives(text: str) -> tuple[str, Optional[str], Optional[float]]:
    """Remove the directive comments from text and return what they set."""
    bg: Optional[str] = None
    opacity: Optional[float] = None
    for match in _DIRECTIVE_RE.finditer(text):
        inner = match.group(1)
        bg_m = _BG_RE.search(inner)
        if bg_m:
            bg = _strip_quotes(bg_m.group("v").strip())
        op_m = _OPACITY_RE.search(inner)
        if op_m:
            opacity = float(op_m.group("v"))
    return _DIRECTIVE_RE.sub("", text), bg, opacity


def _parse_slide(block: str) -> Slide:
    block, bg, opacity = _extract_directives(block)
    lines = block.splitlines()

    title: Optional[str] = None
    body: list[str] = []
    title_taken = False
    for line in lines:
        stripped = line.rstrip()
        if not title_taken and stripped.strip() == "":
            continue
        if not title_taken and stripped.lstrip().startswith("# "):
            title = stripped.lstrip()[2:].strip()
            title_taken = True
            continue
        title_taken = True
        body.append(stripped)

    # Blank lines inside the body are kept; only the ends are trimmed.
    while body and body[0].strip() == "":
        body.pop(0)
    while body and body[-1].strip() == "":
        body.pop()

    slide = Slide(title=title, body_lines=body, background_image=bg)
    if opacity is not None:
        slide.background_opacity = opacity
        slide.__post_init__()  # clamps it
    return slide


def parse_document(text: str, separator: str = DEFAULT_SEPARATOR) -> list[Slide]:
    """Split a document into slides on lines equal to ``separator``."""
    if text is None or text.strip() == "":
        raise ParseError("input document is empty")

    sep = separator.strip()
    blocks: list[str] = []
    current: list[str] = []
    for line in text.splitlines():
        if line.strip() == sep:
            blocks.append("\n".join(current))
            current = []
        else:
            current.append(line)
    blocks.append("\n".join(current))

    slides = [_parse_slide(b) for b in blocks]
    # Empty blocks (say, from a trailing separator) are dropped, but a file of
    # nothing but separators still gives one blank slide.
    non_empty = [s for s in slides if s.title or s.body_lines or s.background_image]
    return non_empty or [Slide()]


def load_config(path: Optional[str]) -> dict[str, Any]:
    """Load the JSON config. No path means an empty config."""
    if not path:
        return {}
    p = Path(path)
    if not p.exists():
        raise ParseError(f"config file not found: {path}")
    try:
        data = json.loads(p.read_text(encoding="utf-8"))
    except json.JSONDecodeError as exc:
        raise ParseError(f"invalid JSON in config {path}: {exc}") from exc
    if not isinstance(data, dict):
        raise ParseError(f"config {path} must be a JSON object")
    return data


def style_from_config(data: dict[str, Any]) -> Style:
    section = data.get("style", {}) or {}
    keys = (
        "background",
        "foreground",
        "font",
        "font_size",
        "size",
        "padding",
        "min_font_size",
        "line_spacing",
        "page_numbers",
    )
    return Style(**{k: section[k] for k in keys if section.get(k) is not None})


def profile_from_config(data: dict[str, Any]) -> Profile:
    section = data.get("profile", {}) or {}
    return Profile(
        image=section.get("image"),
        name=section.get("name"),
        handle=section.get("handle"),
    )


def build_carousel(
    text: str,
    *,
    style: Optional[Style] = None,
    profile: Optional[Profile] = None,
    separator: str = DEFAULT_SEPARATOR,
) -> Carousel:
    return Carousel(
        profile=profile or Profile(),
        style=style or Style(),
        slides=parse_document(text, separator=separator),
    )
