"""Pronoun reflection: "i" -> "you", "my" -> "your" and so on.

It is applied to the words captured from the user's input, not to the
template text around them.
"""

from __future__ import annotations

from typing import Mapping

__all__ = ["reflect"]


def reflect(tokens: list[str], table: Mapping[str, str]) -> list[str]:
    """Lower-case each token and swap it for its entry in ``table``, if any."""
    return [table.get(tok.lower(), tok.lower()) for tok in tokens]
