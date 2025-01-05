"""Result types and exceptions."""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from typing import TYPE_CHECKING

if TYPE_CHECKING:  # pragma: no cover
    from PIL import Image as _ImageModule


class OCRError(Exception):
    """Base class for all OCR errors."""


class UnreadableImageError(OCRError):
    """The image could not be loaded or decoded."""


class NoTextFoundError(OCRError):
    """Text was required but the image has none."""


@dataclass(frozen=True)
class Box:
    """A bounding box in pixels."""

    x: int
    y: int
    w: int
    h: int

    @property
    def x1(self) -> int:
        return self.x + self.w

    @property
    def y1(self) -> int:
        return self.y + self.h

    @property
    def area(self) -> int:
        return self.w * self.h

    def iou(self, other: "Box") -> float:
        """Intersection over union, from 0 to 1."""
        iw = max(0, min(self.x1, other.x1) - max(self.x, other.x))
        ih = max(0, min(self.y1, other.y1) - max(self.y, other.y))
        inter = iw * ih
        union = self.area + other.area - inter
        return inter / union if union > 0 else 0.0

    def as_tuple(self) -> tuple[int, int, int, int]:
        return (self.x, self.y, self.w, self.h)


@dataclass
class CharResult:
    """One recognised character."""

    char: str
    box: Box
    confidence: float


@dataclass
class OCRResult:
    """Everything the engine found in an image."""

    text: str
    chars: list[CharResult] = field(default_factory=list)
    regions: list[Box] = field(default_factory=list)
    skew_angle: float = 0.0

    def to_json(self) -> str:
        payload = {
            "text": self.text,
            "skew_angle": round(self.skew_angle, 3),
            "regions": [r.as_tuple() for r in self.regions],
            "chars": [
                {
                    "char": c.char,
                    "box": c.box.as_tuple(),
                    "confidence": round(c.confidence, 4),
                }
                for c in self.chars
            ],
        }
        return json.dumps(payload, ensure_ascii=False, indent=2)

    def draw_boxes(
        self,
        base_image: "_ImageModule.Image",
        *,
        color: tuple[int, int, int] = (255, 0, 0),
        width: int = 1,
    ) -> "_ImageModule.Image":
        """Return a copy of *base_image* with the character boxes drawn on it."""
        from PIL import Image, ImageDraw

        canvas = base_image.convert("RGB").copy()
        draw = ImageDraw.Draw(canvas)
        for c in self.chars:
            b = c.box
            draw.rectangle([b.x, b.y, b.x1 - 1, b.y1 - 1], outline=color, width=width)
        return canvas
