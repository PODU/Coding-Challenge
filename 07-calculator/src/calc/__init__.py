"""calc: a command-line arithmetic calculator.

Tokenizer, Pratt parser and evaluator over :class:`decimal.Decimal`. Use
:func:`calc.api.evaluate` as the entry point.
"""

from __future__ import annotations

from .api import evaluate
from .errors import CalcError, EvalError, LexError, ParseError

__all__ = ["evaluate", "CalcError", "LexError", "ParseError", "EvalError"]
__version__ = "1.0.0"
