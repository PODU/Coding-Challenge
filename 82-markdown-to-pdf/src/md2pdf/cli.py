"""The md2pdf command.

Errors are reported as a one-line message rather than a traceback, with exit
status 2 (the same status argparse uses for bad flags).
"""

from __future__ import annotations

import argparse
import os
import re
import sys

from . import __version__, _decode
from .render import convert_markdown
from .style import PAGE_SIZES, Style

_HEX_RE = re.compile(r"^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$")


def _hex_color(value: str) -> str:
    if not _HEX_RE.match(value):
        raise argparse.ArgumentTypeError(
            f"invalid hex colour {value!r} (expected e.g. #ffffff)"
        )
    return value


def _positive_float(value: str) -> float:
    try:
        out = float(value)
    except ValueError as exc:
        raise argparse.ArgumentTypeError(f"not a number: {value!r}") from exc
    if out <= 0:
        raise argparse.ArgumentTypeError(f"must be > 0: {value!r}")
    return out


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="md2pdf",
        description="Convert a Markdown file to a paginated PDF (offline).",
    )
    parser.add_argument("input", metavar="INPUT.md", help="Markdown source ('-' = stdin)")
    parser.add_argument("-o", "--output", help="output PDF path (default: INPUT.pdf)")
    parser.add_argument(
        "--page-size", choices=sorted(PAGE_SIZES), default="A4", help="page size"
    )
    parser.add_argument(
        "--margin", type=_positive_float, default=20.0, help="page margin in mm"
    )
    parser.add_argument("--font", default="Helvetica", help="body font family or TTF path")
    parser.add_argument(
        "--font-size", type=_positive_float, default=11.0, help="base font size (pt)"
    )
    parser.add_argument(
        "--heading-scale", type=_positive_float, default=1.0, help="heading size multiplier"
    )
    parser.add_argument("--background", type=_hex_color, default="#ffffff", help="page colour")
    parser.add_argument("--link-color", type=_hex_color, default="#1a0dab", help="link colour")
    parser.add_argument("--code-bg", type=_hex_color, default="#f2f2f2", help="code background")
    parser.add_argument("--title", help="document title for PDF metadata")
    parser.add_argument(
        "--allow-remote-images",
        action="store_true",
        help="download http(s) image URLs (off by default; keeps tool offline)",
    )
    parser.add_argument("-q", "--quiet", action="store_true", help="suppress progress output")
    parser.add_argument("--version", action="version", version=f"md2pdf {__version__}")
    return parser


def _style_from_args(args: argparse.Namespace) -> Style:
    return Style(
        page_size=args.page_size,
        margin_mm=args.margin,
        body_font=args.font,
        body_font_size=args.font_size,
        heading_scale=args.heading_scale,
        background=args.background,
        link_color=args.link_color,
        code_bg=args.code_bg,
        allow_remote_images=args.allow_remote_images,
    )


def _read_input(path: str) -> str:
    if path == "-":
        return _decode(sys.stdin.buffer.read())
    with open(path, "rb") as fh:
        return _decode(fh.read())


def _default_output(input_path: str) -> str:
    if input_path == "-":
        return "output.pdf"
    base, _ = os.path.splitext(input_path)
    return base + ".pdf"


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)

    try:
        text = _read_input(args.input)
    except FileNotFoundError:
        print(f"md2pdf: error: input file not found: {args.input}", file=sys.stderr)
        return 2
    except OSError as exc:
        print(f"md2pdf: error: cannot read input: {exc}", file=sys.stderr)
        return 2

    style = _style_from_args(args)
    output_path = args.output or _default_output(args.input)

    try:
        data = convert_markdown(text, style=style, title=args.title)
    except Exception as exc:
        print(f"md2pdf: error: failed to render PDF: {exc}", file=sys.stderr)
        return 2

    try:
        with open(output_path, "wb") as fh:
            fh.write(data)
    except OSError as exc:
        print(f"md2pdf: error: cannot write output: {exc}", file=sys.stderr)
        return 2

    if not args.quiet:
        print(f"md2pdf: wrote {output_path} ({len(data)} bytes)", file=sys.stderr)
    return 0


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
