"""Interpreter errors."""

from __future__ import annotations


class ForthError(Exception):
    """Base class for compile and runtime errors."""


class StackUnderflow(ForthError):
    """A word needed more values than the stack held."""

    def __init__(self, word: str) -> None:
        self.word = word
        super().__init__(f"{word}: stack underflow")


class UnknownWord(ForthError):
    """A token that is neither a number nor a defined word."""

    def __init__(self, word: str) -> None:
        self.word = word
        # Forth's traditional "foo?" message.
        super().__init__(f"{word}?")


class CompileError(ForthError):
    """Unbalanced or unterminated source."""

    def __init__(self, message: str) -> None:
        super().__init__(f"compile error: {message}")


class ForthExit(Exception):
    """Raised by ``bye`` to stop the REPL or script."""
