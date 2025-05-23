"""The interactive prompt. All console I/O lives here, not in the engine."""

from __future__ import annotations

import argparse
import random
import sys
from typing import Sequence

from .engine import ElizaEngine
from .script import Script, ScriptError

__all__ = ["main"]

_BANNER = r"""
  ______ _      _____ ______      _
 |  ____| |    |_   _|___  /     | |
 | |__  | |      | |    / /  __ _| |
 |  __| | |      | |   / /  / _` | |
 | |____| |____ _| |_ / /__| (_| |_|
 |______|______|_____/_____|\__,_(_)

 ELIZA -- the DOCTOR. (type 'bye', 'quit' or 'exit' to leave)
""".strip(
    "\n"
)


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="eliza",
        description="ELIZA -- a Rogerian psychotherapist chatbot (DOCTOR script).",
    )
    parser.add_argument(
        "--script",
        metavar="PATH",
        help="path to a DOCTOR script JSON file (defaults to the bundled one)",
    )
    parser.add_argument(
        "--seed",
        type=int,
        default=None,
        help="seed for reproducible reassembly when --random is used",
    )
    parser.add_argument(
        "--random",
        action="store_true",
        help="pick reassembly templates randomly instead of round-robin",
    )
    return parser


def main(argv: Sequence[str] | None = None) -> int:
    parser = _build_parser()
    args = parser.parse_args(argv)

    try:
        script = (
            Script.load(args.script) if args.script else Script.load_default()
        )
    except ScriptError as exc:
        print(f"eliza: error: {exc}", file=sys.stderr)
        return 2

    rng = random.Random(args.seed) if args.random else None
    engine = ElizaEngine(
        script, rng=rng, reassembly="random" if args.random else "cycle"
    )

    print(_BANNER)
    print(f"ELIZA: {engine.greeting()}")

    try:
        while True:
            try:
                line = input("YOU:   ")
            except EOFError:
                print()
                break
            if engine.is_quit(line):
                break
            if not line.strip():
                print("ELIZA: Please say something.")
                continue
            print(f"ELIZA: {engine.respond(line)}")
    except KeyboardInterrupt:
        print()

    print(f"ELIZA: {engine.farewell()}")
    return 0


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
