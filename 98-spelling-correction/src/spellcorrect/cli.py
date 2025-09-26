"""Command-line interface.

Usage::

    spellcorrect [--dict PATH | --csv PATH] [--metrics] [--check-only]
                 (--word W | --file F | -)
"""

from __future__ import annotations

import argparse
import sys
import time
from collections.abc import Iterable, Iterator, Sequence

from .corpus import normalize
from .corrector import SpellCorrector, default_corrector

__all__ = ["main", "build_parser"]


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="spellcorrect",
        description="Norvig-style spelling corrector (word / file / stdin).",
    )

    source = parser.add_mutually_exclusive_group()
    source.add_argument(
        "--dict",
        metavar="PATH",
        help="Plain-text corpus to train on (tokenized + counted).",
    )
    source.add_argument(
        "--csv",
        metavar="PATH",
        help='Kaggle-style "word,count" frequency CSV to train on.',
    )

    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--word", metavar="W", help="Correct a single word.")
    mode.add_argument(
        "--file", metavar="F", help="Correct every whitespace token in file F."
    )
    mode.add_argument(
        "-",
        dest="stdin",
        action="store_true",
        help="Read words from stdin (one or many per line).",
    )

    parser.add_argument(
        "--metrics",
        action="store_true",
        help="Print elapsed time and words-per-second throughput.",
    )
    parser.add_argument(
        "--check-only",
        action="store_true",
        help="Lint mode: print only misspelled words; exit 1 if any are found.",
    )
    return parser


def _load_corrector(args: argparse.Namespace) -> SpellCorrector:
    if args.dict:
        return SpellCorrector.from_text_file(args.dict)
    if args.csv:
        return SpellCorrector.from_frequency_csv(args.csv)
    return default_corrector()


def _iter_tokens(args: argparse.Namespace) -> Iterator[str]:
    if args.word is not None:
        yield from args.word.split()
    elif args.file is not None:
        with open(args.file, encoding="utf-8", errors="replace") as handle:
            for line in handle:
                yield from line.split()
    else:
        for line in sys.stdin:
            yield from line.split()


def _process(
    corrector: SpellCorrector,
    tokens: Iterable[str],
    *,
    check_only: bool,
    out,
) -> tuple[int, bool]:
    """Write a ``token<TAB>suggestion`` line per token.

    Returns ``(words_processed, any_misspelled)``.
    """
    processed = 0
    any_misspelled = False
    for token in tokens:
        key = normalize(token)
        if not key:
            continue  # no letters in it
        processed += 1
        suggestion = corrector.correct(token)
        misspelled = suggestion != key
        any_misspelled = any_misspelled or misspelled
        if check_only and not misspelled:
            continue
        out.write(f"{token}\t{suggestion}\n")
    return processed, any_misspelled


def main(argv: Sequence[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)

    try:
        corrector = _load_corrector(args)
    except OSError as exc:
        print(f"spellcorrect: cannot load dictionary: {exc}", file=sys.stderr)
        return 2

    start = time.perf_counter()
    try:
        processed, any_misspelled = _process(
            corrector,
            _iter_tokens(args),
            check_only=args.check_only,
            out=sys.stdout,
        )
    except OSError as exc:
        print(f"spellcorrect: I/O error: {exc}", file=sys.stderr)
        return 2
    elapsed = time.perf_counter() - start

    if args.metrics:
        per_sec = processed / elapsed if elapsed > 0 else 0.0
        print(f"Time: {elapsed * 1000:.6f}ms  {per_sec:.1f} words per second")

    if args.check_only and any_misspelled:
        return 1
    return 0


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
