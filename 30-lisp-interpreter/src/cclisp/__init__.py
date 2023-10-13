"""cclisp - a small Scheme-flavoured Lisp interpreter.

Public API:
    tokenize(text)          -> list[str]
    parse(text)             -> AST            (first top-level form)
    parse_all(text)         -> list[AST]      (all top-level forms)
    standard_env()          -> Environment
    evaluate(x, env)        -> value
    run(text, env=None)     -> value of last form
"""

from __future__ import annotations

from .types import (
    LispError,
    LispSyntaxError,
    LispNameError,
    LispTypeError,
    LispArityError,
    Symbol,
    Lambda,
)
from .tokenizer import tokenize
from .reader import parse, parse_all, read_from_tokens, atom
from .environment import Environment
from .builtins import standard_env
from .evaluator import evaluate, lisp_str

__all__ = [
    "LispError",
    "LispSyntaxError",
    "LispNameError",
    "LispTypeError",
    "LispArityError",
    "Symbol",
    "Lambda",
    "tokenize",
    "parse",
    "parse_all",
    "read_from_tokens",
    "atom",
    "Environment",
    "standard_env",
    "evaluate",
    "lisp_str",
    "run",
]

__version__ = "1.0.0"


def run(text: str, env: "Environment | None" = None):
    """Evaluate every form in *text* and return the last value."""
    if env is None:
        env = standard_env()
    result = False
    for form in parse_all(text):
        result = evaluate(form, env)
    return result
