"""Command-line interface for the OCR engine."""

from __future__ import annotations

import argparse
import sys

from .glyphs import DEFAULT_CHARSET
from .pipeline import OCREngine


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="ocr",
        description="Recognise text in an image (offline template matching).",
    )
    parser.add_argument("image", help="path to the input image (PNG/JPEG/BMP/...)")
    parser.add_argument(
        "--json", action="store_true", help="emit structured JSON instead of text"
    )
    parser.add_argument(
        "--boxes",
        metavar="OUT.png",
        help="also write a copy of the input with the character boxes drawn",
    )
    parser.add_argument(
        "--charset",
        default=DEFAULT_CHARSET,
        help="characters the recogniser may emit (default: digits+letters+punct)",
    )
    parser.add_argument(
        "--font-size", type=int, default=32, help="template render size (default: 32)"
    )
    parser.add_argument(
        "--min-conf",
        type=float,
        default=0.40,
        help="confidence below which a glyph becomes '?' (default: 0.40)",
    )
    parser.add_argument(
        "--no-deskew", action="store_true", help="disable automatic deskew"
    )
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    engine = OCREngine(
        charset=args.charset,
        font_size=args.font_size,
        min_confidence=args.min_conf,
        auto_deskew=not args.no_deskew,
    )
    result = engine.read(args.image)

    if args.boxes:
        from PIL import Image

        base = Image.open(args.image)
        result.draw_boxes(base).save(args.boxes)

    if args.json:
        print(result.to_json())
    else:
        print(result.text)
    return 0


if __name__ == "__main__":  # pragma: no cover
    sys.exit(main())
