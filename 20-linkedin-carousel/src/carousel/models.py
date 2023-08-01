"""Data model: style, profile and slides."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Optional, Tuple, Union

from PIL import ImageColor

# Slide sizes in px.
MIN_SIZE = 270
MAX_SIZE = 4320
DEFAULT_SIZE = 1080
DEFAULT_FONT = "Helvetica"
DEFAULT_FONT_SIZE = 48
DEFAULT_MIN_FONT_SIZE = 24

RGB = Tuple[int, int, int]
ColorLike = Union[str, RGB]


def parse_color(value: ColorLike) -> RGB:
    """Parse a CSS colour name, ``#RGB``/``#RRGGBB``, or an RGB tuple.

    Raises ``ValueError`` if the value isn't a colour.
    """
    if isinstance(value, tuple):
        if len(value) == 3 and all(isinstance(c, int) and 0 <= c <= 255 for c in value):
            return (int(value[0]), int(value[1]), int(value[2]))
        raise ValueError(f"invalid RGB tuple: {value!r}")
    if not isinstance(value, str):
        raise ValueError(f"invalid color: {value!r}")
    try:
        rgb = ImageColor.getrgb(value.strip())
    except ValueError as exc:
        raise ValueError(f"invalid color: {value!r}") from exc
    # getrgb returns RGBA for some inputs.
    return (rgb[0], rgb[1], rgb[2])


def _clamp(value: float, low: float, high: float) -> float:
    return max(low, min(high, value))


@dataclass
class Style:
    """Styling shared by every slide."""

    background: RGB = field(default_factory=lambda: parse_color("#0F172A"))
    foreground: RGB = field(default_factory=lambda: parse_color("#FFFFFF"))
    font: str = DEFAULT_FONT
    font_size: int = DEFAULT_FONT_SIZE
    size: int = DEFAULT_SIZE
    padding: Optional[int] = None
    min_font_size: int = DEFAULT_MIN_FONT_SIZE
    line_spacing: float = 1.3
    page_numbers: bool = True

    def __post_init__(self) -> None:
        # Colours from the config or CLI arrive as strings.
        self.background = parse_color(self.background)
        self.foreground = parse_color(self.foreground)
        self.size = int(_clamp(int(self.size), MIN_SIZE, MAX_SIZE))
        self.font_size = max(1, int(self.font_size))
        self.min_font_size = max(1, int(self.min_font_size))
        if self.min_font_size > self.font_size:
            self.min_font_size = self.font_size

    @property
    def pad(self) -> int:
        """Inner margin in px; 8% of the slide unless set explicitly."""
        if self.padding is not None:
            return int(self.padding)
        return round(self.size * 0.08)


@dataclass
class Profile:
    """The name and picture shown at the top left of every slide."""

    image: Optional[str] = None
    name: Optional[str] = None
    handle: Optional[str] = None

    @property
    def has_content(self) -> bool:
        return bool(self.image or self.name or self.handle)


@dataclass
class Slide:
    """One page of the carousel."""

    title: Optional[str] = None
    body_lines: list[str] = field(default_factory=list)
    background_image: Optional[str] = None
    background_opacity: float = 0.3

    def __post_init__(self) -> None:
        self.background_opacity = float(_clamp(self.background_opacity, 0.0, 1.0))


@dataclass
class Carousel:
    """The whole document."""

    profile: Profile = field(default_factory=Profile)
    style: Style = field(default_factory=Style)
    slides: list[Slide] = field(default_factory=list)

    @property
    def page_count(self) -> int:
        return len(self.slides)
