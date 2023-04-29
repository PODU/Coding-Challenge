"""Calculator errors. ``pos`` is the source column used for the caret in CLI output."""

from __future__ import annotations


class CalcError(Exception):
    def __init__(self, message: str, pos: int | None = None) -> None:
        super().__init__(message)
        self.message = message
        self.pos = pos

    def __str__(self) -> str:  # pragma: no cover - trivial
        return self.message


class LexError(CalcError):
    """Unknown character or malformed number."""


class ParseError(CalcError):
    """Syntax error: bad token, missing operand, unbalanced or empty parens."""


class EvalError(CalcError):
    """Division by zero, domain error, overflow, or unknown name."""
