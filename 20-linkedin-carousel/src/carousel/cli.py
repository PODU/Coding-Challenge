"""Command-line entry point."""

from __future__ import annotations

import argparse
import sys
from pathlib import Path
from typing import Optional, Sequence

from . import __version__
from .models import Carousel, Profile, Style, parse_color
from .parser import (
    ParseError,
    load_config,
    parse_document,
    profile_from_config,
    style_from_config,
)
from .renderer import RenderError, render_carousel


def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        prog="carousel",
        description="Turn Markdown/text into a multi-slide LinkedIn PDF carousel.",
    )
    p.add_argument("input", help="Path to a Markdown/text carousel source file.")
    p.add_argument("-o", "--output", help="Output PDF path. Default: <input stem>.pdf")
    p.add_argument(
        "-c",
        "--config",
        help="Style/profile JSON. Default: style.json beside input if present.",
    )
    p.add_argument("--size", type=int, help="Square slide size in px (default 1080, max 4320).")
    p.add_argument("--bg", help="Background color (name or #RRGGBB).")
    p.add_argument("--fg", help="Foreground/text color.")
    p.add_argument("--font", help="Font face (core name or path to a .ttf).")
    p.add_argument("--font-size", type=int, help="Base body font size in px.")
    p.add_argument("--profile-image", help="Profile picture (top-left).")
    p.add_argument("--name", help="Display name.")
    p.add_argument("--handle", help="Handle, e.g. @janedev.")
    p.add_argument(
        "--slide-separator",
        default="---",
        help="Slide delimiter (default: a line equal to '---').",
    )
    page = p.add_mutually_exclusive_group()
    page.add_argument(
        "--page-numbers",
        dest="page_numbers",
        action="store_true",
        default=None,
        help="Show 'n / N' page indicators (default on).",
    )
    page.add_argument(
        "--no-page-numbers",
        dest="page_numbers",
        action="store_false",
        help="Hide page indicators.",
    )
    p.add_argument("-q", "--quiet", action="store_true", help="Suppress warnings.")
    p.add_argument("--version", action="version", version=f"carousel {__version__}")
    return p


def _default_config_path(input_path: Path) -> Optional[str]:
    candidate = input_path.parent / "style.json"
    return str(candidate) if candidate.exists() else None


def _apply_overrides(style: Style, profile: Profile, args: argparse.Namespace) -> None:
    if args.size is not None:
        style.size = args.size
    if args.bg is not None:
        style.background = parse_color(args.bg)
    if args.fg is not None:
        style.foreground = parse_color(args.fg)
    if args.font is not None:
        style.font = args.font
    if args.font_size is not None:
        style.font_size = args.font_size
    if args.page_numbers is not None:
        style.page_numbers = args.page_numbers
    # Normalise colours and clamp sizes again after the overrides.
    style.__post_init__()

    if args.profile_image is not None:
        profile.image = args.profile_image
    if args.name is not None:
        profile.name = args.name
    if args.handle is not None:
        profile.handle = args.handle


def _resolve_image(path: Optional[str], base: Path) -> Optional[str]:
    if not path:
        return path
    p = Path(path)
    if p.is_absolute() or p.exists():
        return path
    candidate = base / path
    return str(candidate) if candidate.exists() else path


def _resolve_images(profile: Profile, slides, base: Path) -> None:
    profile.image = _resolve_image(profile.image, base)
    for slide in slides:
        slide.background_image = _resolve_image(slide.background_image, base)


def main(argv: Optional[Sequence[str]] = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)

    input_path = Path(args.input)
    if not input_path.exists():
        print(f"error: input file not found: {args.input}", file=sys.stderr)
        return 1

    try:
        text = input_path.read_text(encoding="utf-8")
    except OSError as exc:
        print(f"error: cannot read input: {exc}", file=sys.stderr)
        return 1

    config_path = args.config or _default_config_path(input_path)
    try:
        config = load_config(config_path)
        style = style_from_config(config)
        profile = profile_from_config(config)
        _apply_overrides(style, profile, args)
        slides = parse_document(text, separator=args.slide_separator)
    except (ParseError, ValueError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1

    # Relative image paths are relative to the input file, not the cwd.
    _resolve_images(profile, slides, input_path.parent)

    output_path = args.output or str(input_path.with_suffix(".pdf"))
    carousel = Carousel(profile=profile, style=style, slides=slides)

    try:
        render_carousel(carousel, output_path, quiet=args.quiet)
    except (RenderError, ValueError, OSError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1

    if not args.quiet:
        print(f"wrote {output_path} ({carousel.page_count} slides)")
    return 0


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
