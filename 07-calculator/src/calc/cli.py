"""Command-line interface: one expression as an argument, or a stdin/REPL loop.

Exit codes::

    0   success
    1   evaluation error (e.g. division by zero)
    2   parse/lex error (invalid syntax)
    64  usage error (EX_USAGE)
"""

from __future__ import annotations

import argparse
import sys
from typing import IO, Sequence

from . import __version__
from .api import evaluate, format_result
from .errors import CalcError, LexError, ParseError

EXIT_OK = 0
EXIT_EVAL = 1
EXIT_SYNTAX = 2
EXIT_USAGE = 64


def _exit_code_for(error: CalcError) -> int:
    if isinstance(error, (LexError, ParseError)):
        return EXIT_SYNTAX
    return EXIT_EVAL


def _render_error(source: str, error: CalcError) -> str:
    """Format an error, with a caret under the offending column when known."""
    label = type(error).__name__
    if error.pos is not None and 0 <= error.pos <= len(source):
        caret = " " * error.pos + "^"
        return f"{source}\n{caret} {label}: {error.message}"
    return f"{label}: {error.message}"


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="calc",
        description="Evaluate arithmetic expressions with correct precedence.",
    )
    parser.add_argument(
        "expression",
        nargs="?",
        help="expression to evaluate; omit for stdin/REPL mode",
    )
    parser.add_argument(
        "--precision",
        type=int,
        metavar="N",
        help="round output to N significant digits (default: full Decimal)",
    )
    parser.add_argument(
        "--quiet",
        action="store_true",
        help="suppress the 'calc> ' prompt (script-friendly REPL)",
    )
    parser.add_argument(
        "--version",
        action="version",
        version=f"calc {__version__}",
    )
    return parser


def _evaluate_line(
    source: str, precision: int | None, out: IO[str], err: IO[str]
) -> int:
    """Evaluate one line, print the result or error, and return the exit code."""
    try:
        result = evaluate(source)
    except CalcError as exc:
        print(_render_error(source, exc), file=err)
        return _exit_code_for(exc)
    print(format_result(result, precision), file=out)
    return EXIT_OK


def _run_repl(
    precision: int | None,
    quiet: bool,
    in_stream: IO[str],
    out: IO[str],
    err: IO[str],
) -> int:
    """Evaluate one expression per line until EOF, ``quit`` or ``exit``.

    The exit code is that of the last non-blank line.
    """
    prompt = "" if quiet else "calc> "
    last_code = EXIT_OK
    interactive = in_stream.isatty()

    while True:
        if prompt and interactive:
            out.write(prompt)
            out.flush()
        line = in_stream.readline()
        if line == "":
            if interactive and prompt:
                out.write("\n")
            break
        text = line.strip()
        if not text:
            continue
        if text.lower() in ("quit", "exit"):
            break
        last_code = _evaluate_line(text, precision, out, err)

    return last_code


def main(argv: Sequence[str] | None = None) -> int:
    """Evaluate the argument if given, otherwise read lines from stdin."""
    args = _build_parser().parse_args(argv)

    if args.precision is not None and args.precision < 1:
        print("error: --precision must be a positive integer", file=sys.stderr)
        return EXIT_USAGE

    if args.expression is not None:
        if args.expression.strip() == "":
            print("error: empty expression", file=sys.stderr)
            return EXIT_USAGE
        return _evaluate_line(
            args.expression, args.precision, sys.stdout, sys.stderr
        )

    return _run_repl(
        args.precision, args.quiet, sys.stdin, sys.stdout, sys.stderr
    )


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
