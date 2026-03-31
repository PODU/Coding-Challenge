"""Command-line entry point."""

from __future__ import annotations

import argparse
import sys
from collections.abc import Sequence

from . import __version__
from .judge import make_judge
from .report import render_json, render_text
from .scorer import aggregate
from .tokenize import build_document

EXIT_OK = 0
EXIT_EMPTY = 1
EXIT_USAGE = 2


def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        prog="aidetector",
        description="Estimate how likely a block of text was written by an AI (LLM). "
        "Rule-based + statistical, deterministic and offline by default.",
    )
    p.add_argument("text", nargs="?", help="Text to analyse (positional).")
    p.add_argument("-f", "--file", help="Read text from a file instead.")
    p.add_argument(
        "--format",
        choices=("text", "json"),
        default="text",
        help="Output format (default: text).",
    )
    p.add_argument(
        "--threshold-human",
        type=int,
        default=30,
        help="Upper bound (exclusive) for 'likely human' (default: 30).",
    )
    p.add_argument(
        "--threshold-ai",
        type=int,
        default=60,
        help="Lower bound (inclusive) for 'likely AI' (default: 60).",
    )
    p.add_argument(
        "--explain",
        action="store_true",
        help="Verbose report: list every finding with evidence.",
    )
    p.add_argument(
        "--judge",
        choices=("none", "mock", "anthropic"),
        default="none",
        help="Optional second opinion. 'anthropic' needs ANTHROPIC_API_KEY + "
        "network; 'mock' is offline. Default: none.",
    )
    p.add_argument("--version", action="version", version=f"%(prog)s {__version__}")
    return p


def _read_input(args: argparse.Namespace) -> str:
    if args.file is not None:
        try:
            with open(args.file, encoding="utf-8") as fh:
                return fh.read()
        except OSError as exc:
            raise _UsageError(f"cannot read file {args.file!r}: {exc}") from exc
    if args.text is not None:
        return args.text
    if not sys.stdin.isatty():
        return sys.stdin.read()
    return ""


class _UsageError(Exception):
    pass


def main(argv: Sequence[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)

    try:
        raw = _read_input(args)
    except _UsageError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return EXIT_USAGE

    if not raw or not raw.strip():
        print(
            "No text to analyse. Provide TEXT, --file PATH, or pipe via STDIN.",
            file=sys.stderr,
        )
        return EXIT_EMPTY

    doc = build_document(raw)
    result = aggregate(
        doc,
        threshold_human=args.threshold_human,
        threshold_ai=args.threshold_ai,
    )

    judge_opinion = None
    if args.judge != "none":
        try:
            judge = make_judge(args.judge)
            if judge is not None:
                judge_opinion = judge.assess(doc.text)
        except (RuntimeError, ValueError) as exc:
            print(f"error: judge unavailable: {exc}", file=sys.stderr)
            return EXIT_USAGE

    if args.format == "json":
        print(render_json(result, judge_opinion))
    else:
        print(render_text(result, explain=args.explain, judge=judge_opinion))
    return EXIT_OK


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
