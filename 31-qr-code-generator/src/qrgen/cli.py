"""Command-line front end for qrgen."""
from __future__ import annotations

import argparse
import sys
from typing import List, Optional

from . import QRCode, __version__, encode
from .analyze import CapacityError, ModeError


def _build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        prog="qrgen",
        description="Generate a QR code from text (ISO/IEC 18004).",
    )
    p.add_argument("text", nargs="?", help="Text to encode (else --input or stdin).")
    p.add_argument("-o", "--output", help="Output file (.png/.svg/.txt selects format).")
    p.add_argument(
        "-f", "--format",
        choices=["png", "svg", "terminal", "txt"],
        help="Force renderer (default: inferred from --output, else terminal).",
    )
    p.add_argument("-e", "--ec", choices=["L", "M", "Q", "H"], default="M",
                   help="Error-correction level (default: M).")
    p.add_argument(
        "-m", "--mode",
        choices=["auto", "numeric", "alphanumeric", "byte", "kanji"],
        default="auto", help="Encoding mode (default: auto).",
    )
    p.add_argument("-v", "--version", default="auto",
                   help="Symbol version 'auto' or 1..40 (default: auto).")
    p.add_argument("--mask", default="auto",
                   help="Mask pattern 'auto' or 0..7 (default: auto).")
    p.add_argument("-s", "--scale", type=int, default=10,
                   help="Pixels/units per module (default: 10).")
    p.add_argument("-b", "--border", type=int, default=4,
                   help="Quiet-zone width in modules (default: 4).")
    p.add_argument("--input", help="Read payload text from FILE (UTF-8).")
    p.add_argument("--invert", action="store_true",
                   help="Swap dark/light modules.")
    p.add_argument("--qrversion", action="version",
                   version=f"qrgen {__version__}")
    return p


def _resolve_text(args: argparse.Namespace) -> str:
    if args.text is not None:
        return args.text
    if args.input:
        with open(args.input, "r", encoding="utf-8") as fh:
            return fh.read().rstrip("\n")
    if not sys.stdin.isatty():
        return sys.stdin.read().rstrip("\n")
    return ""


def _resolve_format(args: argparse.Namespace) -> str:
    if args.format:
        return "terminal" if args.format == "txt" else args.format
    if args.output:
        lower = args.output.lower()
        if lower.endswith(".png"):
            return "png"
        if lower.endswith(".svg"):
            return "svg"
        if lower.endswith(".txt"):
            return "terminal"
    return "terminal"


def _force_utf8_stdio() -> None:
    """Switch stdout/stderr to UTF-8 where possible.

    Windows consoles on a legacy code page raise UnicodeEncodeError on the
    half-block characters otherwise.
    """
    for stream in (sys.stdout, sys.stderr):
        reconfigure = getattr(stream, "reconfigure", None)
        if reconfigure is not None:
            try:
                reconfigure(encoding="utf-8")
            except (ValueError, OSError):  # pragma: no cover
                pass


def main(argv: Optional[List[str]] = None) -> int:
    _force_utf8_stdio()
    parser = _build_parser()
    args = parser.parse_args(argv)

    version: Optional[int]
    if args.version == "auto":
        version = None
    else:
        try:
            version = int(args.version)
            if not 1 <= version <= 40:
                raise ValueError
        except ValueError:
            parser.error("--version must be 'auto' or an integer 1..40")

    mask: Optional[int]
    if args.mask == "auto":
        mask = None
    else:
        try:
            mask = int(args.mask)
            if not 0 <= mask <= 7:
                raise ValueError
        except ValueError:
            parser.error("--mask must be 'auto' or an integer 0..7")

    mode = None if args.mode == "auto" else args.mode
    text = _resolve_text(args)

    fmt = _resolve_format(args)
    if fmt in ("png", "svg") and args.scale <= 0:
        print("error: --scale must be > 0 for image output", file=sys.stderr)
        return 1
    if fmt in ("png", "svg") and args.border < 0:
        print("error: --border must be >= 0", file=sys.stderr)
        return 1

    try:
        qr: QRCode = encode(text, ec=args.ec, mode=mode, version=version, mask=mask)
    except (CapacityError, ModeError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1

    try:
        if fmt == "png":
            if not args.output:
                print("error: --output is required for PNG", file=sys.stderr)
                return 1
            qr.to_png(args.output, border=args.border, scale=args.scale,
                      invert=args.invert)
        elif fmt == "svg":
            svg = qr.to_svg(border=args.border, scale=args.scale, invert=args.invert)
            if args.output:
                with open(args.output, "w", encoding="utf-8") as fh:
                    fh.write(svg)
            else:
                sys.stdout.write(svg)
        else:
            text_out = qr.to_terminal(border=args.border, invert=args.invert)
            if args.output:
                with open(args.output, "w", encoding="utf-8") as fh:
                    fh.write(text_out + "\n")
            else:
                sys.stdout.write(text_out + "\n")
    except (RuntimeError, ValueError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1

    return 0


if __name__ == "__main__":  # pragma: no cover
    sys.exit(main())
