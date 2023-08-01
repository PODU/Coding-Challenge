"""Turn Markdown or plain text into a square, multi-page PDF for LinkedIn."""

from __future__ import annotations

from .models import Carousel, Profile, Slide, Style, parse_color
from .parser import ParseError, build_carousel, load_config, parse_document
from .layout import ProfileBox, SlideLayout, TextLine, layout_slide
from .renderer import RenderError, render_carousel

__all__ = [
    "Carousel",
    "Profile",
    "Slide",
    "Style",
    "parse_color",
    "ParseError",
    "build_carousel",
    "load_config",
    "parse_document",
    "ProfileBox",
    "SlideLayout",
    "TextLine",
    "layout_slide",
    "RenderError",
    "render_carousel",
]

__version__ = "1.0.0"
