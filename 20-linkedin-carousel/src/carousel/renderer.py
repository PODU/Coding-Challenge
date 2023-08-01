"""PDF output with reportlab, using Pillow for the images.

Page size in points equals the slide size in pixels, so 1px is 1pt.
"""

from __future__ import annotations

import sys
from io import BytesIO
from pathlib import Path
from typing import Callable, Optional

from PIL import Image, ImageDraw
from reportlab.lib.utils import ImageReader
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.pdfgen import canvas

from .layout import layout_slide
from .models import RGB, Carousel, Slide, Style

# The 14 standard PDF fonts, which need no embedding.
_CORE_FONTS = {
    "Helvetica",
    "Helvetica-Bold",
    "Helvetica-Oblique",
    "Helvetica-BoldOblique",
    "Times-Roman",
    "Times-Bold",
    "Times-Italic",
    "Times-BoldItalic",
    "Courier",
    "Courier-Bold",
    "Courier-Oblique",
    "Courier-BoldOblique",
    "Symbol",
    "ZapfDingbats",
}

_FALLBACK_FONT = "Helvetica"


class RenderError(RuntimeError):
    """The PDF could not be written."""


def _warn(message: str, quiet: bool) -> None:
    if not quiet:
        print(f"warning: {message}", file=sys.stderr)


def _resolve_font(font: str, quiet: bool) -> str:
    """Return a reportlab font name for ``font``, registering it if it's a file."""
    if font in _CORE_FONTS:
        return font
    path = Path(font)
    if path.exists() and path.suffix.lower() in (".ttf", ".otf"):
        name = "CarouselFont_" + path.stem
        try:
            if name not in pdfmetrics.getRegisteredFontNames():
                pdfmetrics.registerFont(TTFont(name, str(path)))
            return name
        except Exception as exc:  # noqa: BLE001 - reportlab raises broad errors
            _warn(f"could not load font {font!r} ({exc}); using {_FALLBACK_FONT}", quiet)
            return _FALLBACK_FONT
    _warn(f"unknown font {font!r}; using {_FALLBACK_FONT}", quiet)
    return _FALLBACK_FONT


def _rgb01(color: RGB) -> tuple[float, float, float]:
    return (color[0] / 255.0, color[1] / 255.0, color[2] / 255.0)


def _cover_crop(img: Image.Image, size: int) -> Image.Image:
    """Scale and centre-crop an image to fill a square."""
    img = img.convert("RGB")
    w, h = img.size
    scale = max(size / w, size / h)
    new_w, new_h = max(1, round(w * scale)), max(1, round(h * scale))
    img = img.resize((new_w, new_h), Image.LANCZOS)
    left = (new_w - size) // 2
    top = (new_h - size) // 2
    return img.crop((left, top, left + size, top + size))


def _build_background(style: Style, slide: Slide, quiet: bool) -> Optional[Image.Image]:
    """Return the slide's background image blended over the background
    colour, or None if it has none."""
    if not slide.background_image:
        return None
    path = Path(slide.background_image)
    if not path.exists():
        _warn(f"background image not found: {slide.background_image}; skipping", quiet)
        return None
    try:
        with Image.open(path) as raw:
            fg = _cover_crop(raw, style.size)
    except Exception as exc:  # noqa: BLE001
        _warn(f"could not read background image {slide.background_image} ({exc}); skipping", quiet)
        return None
    base = Image.new("RGB", (style.size, style.size), style.background)
    opacity = max(0.0, min(1.0, slide.background_opacity))
    return Image.blend(base, fg, opacity)


def _circular_profile(path_str: str, diameter: int, quiet: bool) -> Optional[Image.Image]:
    """Load the profile picture cropped to a circle."""
    path = Path(path_str)
    if not path.exists():
        _warn(f"profile image not found: {path_str}; skipping", quiet)
        return None
    try:
        with Image.open(path) as raw:
            square = _cover_crop(raw, diameter).convert("RGBA")
    except Exception as exc:  # noqa: BLE001
        _warn(f"could not read profile image {path_str} ({exc}); skipping", quiet)
        return None
    mask = Image.new("L", (diameter, diameter), 0)
    draw = ImageDraw.Draw(mask)
    draw.ellipse((0, 0, diameter - 1, diameter - 1), fill=255)
    square.putalpha(mask)
    return square


def _measure_factory(font_name: str) -> Callable[[str, float], float]:
    def measure(text: str, size: float) -> float:
        return pdfmetrics.stringWidth(text, font_name, size)
    return measure


def render_carousel(carousel: Carousel, output_path: str, quiet: bool = False) -> str:
    """Write ``carousel`` to ``output_path`` and return the path."""
    style = carousel.style
    profile = carousel.profile
    font_name = _resolve_font(style.font, quiet)
    measure = _measure_factory(font_name)

    out = Path(output_path)
    if out.parent and not out.parent.exists():
        raise RenderError(f"output directory does not exist: {out.parent}")

    profile_img: Optional[Image.Image] = None
    profile_diameter = round(style.size * 0.085)
    if profile.image:
        profile_img = _circular_profile(profile.image, profile_diameter, quiet)

    buffer = BytesIO()
    c = canvas.Canvas(buffer, pagesize=(style.size, style.size))
    fg = _rgb01(style.foreground)

    for i, slide in enumerate(carousel.slides):
        layout = layout_slide(slide, style, measure, profile=profile)
        size = style.size

        background = _build_background(style, slide, quiet)
        if background is not None:
            c.drawImage(ImageReader(background), 0, 0, size, size)
        else:
            c.setFillColorRGB(*_rgb01(style.background))
            c.rect(0, 0, size, size, stroke=0, fill=1)

        box = layout.profile
        if box is not None and box.has_image and profile_img is not None:
            c.drawImage(
                ImageReader(profile_img),
                box.x,
                size - box.y - box.diameter,
                box.diameter,
                box.diameter,
                mask="auto",
            )

        c.setFillColorRGB(*fg)
        for line in layout.lines:
            c.setFont(font_name, line.size)
            c.drawString(line.x, size - line.baseline, line.text)

        if box is not None:
            c.setFillColorRGB(*fg)
            if box.name:
                c.setFont(font_name, box.name_size)
                c.drawString(box.name_x, size - box.name_baseline, box.name)
            if box.handle:
                c.setFont(font_name, box.handle_size)
                c.drawString(box.name_x, size - box.handle_baseline, box.handle)

        if style.page_numbers:
            label = f"{i + 1} / {carousel.page_count}"
            ind_size = round(size * 0.022)
            c.setFont(font_name, ind_size)
            c.setFillColorRGB(*fg)
            text_w = pdfmetrics.stringWidth(label, font_name, ind_size)
            c.drawString(size - style.pad - text_w, style.pad * 0.4, label)

        c.showPage()

    c.save()
    out.write_bytes(buffer.getvalue())
    return str(out)
