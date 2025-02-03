"""Fonts, sizes, colours and page layout for the PDF."""

from __future__ import annotations

from dataclasses import dataclass, field

# In points, the same values as reportlab.lib.pagesizes.
PAGE_SIZES: dict[str, tuple[float, float]] = {
    "A4": (595.2755905511812, 841.8897637795277),
    "Letter": (612.0, 792.0),
}

MM_TO_PT = 72.0 / 25.4

# Font families every PDF reader has built in.
_BUILTIN_FONTS = {"Helvetica", "Times-Roman", "Courier"}

_FONT_VARIANTS: dict[str, dict[str, str]] = {
    "Helvetica": {
        "normal": "Helvetica",
        "bold": "Helvetica-Bold",
        "italic": "Helvetica-Oblique",
        "bolditalic": "Helvetica-BoldOblique",
    },
    "Times-Roman": {
        "normal": "Times-Roman",
        "bold": "Times-Bold",
        "italic": "Times-Italic",
        "bolditalic": "Times-BoldItalic",
    },
    "Courier": {
        "normal": "Courier",
        "bold": "Courier-Bold",
        "italic": "Courier-Oblique",
        "bolditalic": "Courier-BoldOblique",
    },
}


@dataclass
class Style:
    page_size: str = "A4"
    margin_mm: float = 20.0

    body_font: str = "Helvetica"
    body_font_size: float = 11.0
    leading_ratio: float = 1.35  # line height as a multiple of the font size

    heading_scale: float = 1.0
    # H1..H6 sizes relative to the body text, before heading_scale.
    heading_ratios: tuple[float, ...] = (2.0, 1.6, 1.3, 1.15, 1.05, 1.0)

    mono_font: str = "Courier"
    code_font_size: float = 9.5

    background: str = "#ffffff"
    text_color: str = "#000000"
    link_color: str = "#1a0dab"
    code_bg: str = "#f2f2f2"
    code_color: str = "#24292e"

    footer_text: str = "Page {page} of {total}"
    allow_remote_images: bool = True  # the CLI turns this off unless asked

    # Filled in by the renderer.
    _warnings: list[str] = field(default_factory=list, repr=False)

    @property
    def page_size_pt(self) -> tuple[float, float]:
        try:
            return PAGE_SIZES[self.page_size]
        except KeyError as exc:  # pragma: no cover - guarded by CLI
            raise ValueError(f"Unknown page size: {self.page_size!r}") from exc

    @property
    def margin_pt(self) -> float:
        return self.margin_mm * MM_TO_PT

    @property
    def body_leading(self) -> float:
        return self.body_font_size * self.leading_ratio

    def heading_size(self, level: int) -> float:
        idx = max(1, min(level, len(self.heading_ratios))) - 1
        return self.body_font_size * self.heading_ratios[idx] * self.heading_scale

    def font_name(self, *, bold: bool = False, italic: bool = False) -> str:
        """Return the font for this style, e.g. "Helvetica-BoldOblique"."""
        family = self.body_font if self.body_font in _FONT_VARIANTS else "Helvetica"
        if bold and italic:
            key = "bolditalic"
        elif bold:
            key = "bold"
        elif italic:
            key = "italic"
        else:
            key = "normal"
        return _FONT_VARIANTS[family][key]

    @staticmethod
    def is_builtin_font(name: str) -> bool:
        return name in _BUILTIN_FONTS
