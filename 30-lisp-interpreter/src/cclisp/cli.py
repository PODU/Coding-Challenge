"""The REPL and the file runner."""

from __future__ import annotations

import sys

from .builtins import standard_env
from .environment import Environment
from .evaluator import evaluate, lisp_str
from .reader import parse_all, read_from_tokens
from .tokenizer import tokenize
from .types import LispError, LispSyntaxError

PROMPT = "cclisp> "
CONTINUATION = "    ... "


def run_file(path: str, env: Environment | None = None) -> object:
    """Evaluate every form in the file at *path*; return the last value."""
    if env is None:
        env = standard_env()
    with open(path, "r", encoding="utf-8") as fh:
        source = fh.read()
    result: object = False
    for form in parse_all(source):
        result = evaluate(form, env)
    return result


def _complete(source: str) -> bool:
    """Whether *source* is complete enough to evaluate, or the REPL should keep
    reading lines."""
    try:
        tokens = tokenize(source)
    except LispSyntaxError:
        # most likely an unterminated string
        return False
    depth = 0
    for tok in tokens:
        if tok == "(":
            depth += 1
        elif tok == ")":
            depth -= 1
            if depth < 0:
                return True  # stray ')': let the reader report it
    return depth <= 0


def repl(env: Environment | None = None, *, input_fn=input, out=None) -> None:
    if env is None:
        env = standard_env()
    if out is None:
        out = sys.stdout

    buffer = ""
    while True:
        prompt = CONTINUATION if buffer else PROMPT
        try:
            line = input_fn(prompt)
        except EOFError:
            break
        except KeyboardInterrupt:
            out.write("\n")
            buffer = ""
            continue

        buffer = f"{buffer}\n{line}" if buffer else line
        if not buffer.strip():
            buffer = ""
            continue
        if not _complete(buffer):
            continue

        source, buffer = buffer, ""
        try:
            tokens = tokenize(source)
            while tokens:
                form = read_from_tokens(tokens)
                value = evaluate(form, env)
                out.write(lisp_str(value, repr_strings=True) + "\n")
        except LispError as exc:
            out.write(f"{type(exc).__name__}: {exc}\n")
        except RecursionError:
            out.write("Error: maximum recursion depth exceeded\n")


def main(argv: list[str] | None = None) -> int:
    if argv is None:
        argv = sys.argv[1:]

    sys.setrecursionlimit(100_000)

    if argv:
        path = argv[0]
        try:
            run_file(path)
        except FileNotFoundError:
            sys.stderr.write(f"cclisp: cannot open file: {path}\n")
            return 2
        except LispError as exc:
            sys.stderr.write(f"{type(exc).__name__}: {exc}\n")
            return 1
        return 0

    repl()
    return 0


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
