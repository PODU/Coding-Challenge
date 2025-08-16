"""The REPL and the script runner."""

from __future__ import annotations

import sys
from typing import Sequence

from .errors import ForthError, ForthExit
from .interpreter import Interpreter


def repl(interp: Interpreter | None = None) -> int:
    """Run the REPL until ``bye`` or end of input. Errors are printed and the
    session carries on."""
    interp = interp if interp is not None else Interpreter()
    while True:
        try:
            line = input(f"{interp.stack_summary()} ok> ")
        except EOFError:
            print()
            return 0
        except KeyboardInterrupt:
            print()
            continue
        try:
            interp.run(line)
        except ForthExit:
            return 0
        except ForthError as exc:
            print(exc)


def run_file(path: str, interp: Interpreter | None = None) -> int:
    """Run a Forth file and return the exit code."""
    interp = interp if interp is not None else Interpreter()
    try:
        with open(path, encoding="utf-8") as fh:
            source = fh.read()
    except OSError as exc:
        print(f"forth: cannot open {path}: {exc}", file=sys.stderr)
        return 1
    try:
        interp.run(source)
    except ForthExit:
        return 0
    except ForthError as exc:
        print(exc, file=sys.stderr)
        return 1
    return 0


def main(argv: Sequence[str] | None = None) -> int:
    """The ``forth`` command: run a file if one is given, else start the REPL."""
    args = list(sys.argv[1:] if argv is None else argv)
    if args:
        return run_file(args[0])
    return repl()


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
