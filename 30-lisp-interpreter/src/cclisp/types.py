"""Core data types and the error hierarchy for cclisp."""

from __future__ import annotations

from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from .environment import Environment


class LispError(Exception):
    """Base class for every error raised by the interpreter."""


class LispSyntaxError(LispError):
    """Tokenizer / reader error (unbalanced parens, unterminated string, ...)."""


class LispNameError(LispError):
    """Reference to an unbound symbol."""


class LispTypeError(LispError):
    """A value of the wrong type was supplied to an operator/builtin."""


class LispArityError(LispError):
    """A procedure was called with the wrong number of arguments."""


class Symbol(str):
    """A Lisp symbol. Subclassing str lets symbols be environment keys while
    staying distinguishable from string literals, which are plain str."""

    __slots__ = ()

    def __repr__(self) -> str:  # pragma: no cover - debugging aid
        return f"Symbol({str.__repr__(self)})"


class Lambda:
    """A user-defined procedure and the environment it closes over."""

    __slots__ = ("params", "body", "env", "name")

    def __init__(
        self,
        params: list[Symbol],
        body: object,
        env: "Environment",
        name: str = "lambda",
    ) -> None:
        self.params = params
        self.body = body  # a single form; multi-expression bodies come wrapped in begin
        self.env = env
        self.name = name

    def __repr__(self) -> str:
        return f"#<procedure {self.name} ({' '.join(self.params)})>"
