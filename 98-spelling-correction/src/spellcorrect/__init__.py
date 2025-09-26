"""Spelling correction in the style of Peter Norvig's corrector.

Example::

    from spellcorrect import SpellCorrector
    sc = SpellCorrector.from_text_file("big.txt")
    sc.correct("speling")   # -> "spelling"
"""

from __future__ import annotations

from .corpus import (
    count_text,
    load_frequency_csv,
    load_text_file,
    normalize,
    tokenize,
)
from .corrector import ALPHABET, SpellCorrector, default_corrector

__all__ = [
    "SpellCorrector",
    "default_corrector",
    "ALPHABET",
    "normalize",
    "tokenize",
    "count_text",
    "load_text_file",
    "load_frequency_csv",
]

__version__ = "1.0.0"
