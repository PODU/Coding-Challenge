"""``evaluate`` (string to Decimal, no I/O) and ``format_result``."""

from __future__ import annotations

from decimal import Context, Decimal

from .evaluator import evaluate_ast
from .lexer import tokenize
from .parser import parse


def evaluate(source: str) -> Decimal:
    """Evaluate an expression such as ``"(1 + 1) * 5"``.

    Raises LexError, ParseError or EvalError (all subclasses of CalcError).
    """
    return evaluate_ast(parse(tokenize(source)))


def format_result(value: Decimal, precision: int | None = None) -> str:
    """Format a result for display.

    Integers print without a trailing ``.0`` and fractions have trailing zeros
    trimmed. ``precision`` rounds to that many significant digits first.
    """
    if precision is not None:
        value = Context(prec=precision).create_decimal((+value).normalize())

    if value == 0:
        # Avoid "-0" and exponent artefacts.
        return "0"

    if value == value.to_integral_value():
        return str(value.quantize(Decimal(1)))

    text = format(value.normalize(), "f")
    if "." in text:
        text = text.rstrip("0").rstrip(".")
    return text
