"""A small Forth interpreter.

Use :class:`Interpreter` directly, or :func:`main` and :func:`repl` for the
command line.
"""

from __future__ import annotations

from .cli import main, repl, run_file
from .errors import (
    CompileError,
    ForthError,
    ForthExit,
    StackUnderflow,
    UnknownWord,
)
from .interpreter import Interpreter

__all__ = [
    "Interpreter",
    "ForthError",
    "StackUnderflow",
    "UnknownWord",
    "CompileError",
    "ForthExit",
    "main",
    "repl",
    "run_file",
]

__version__ = "1.0.0"
