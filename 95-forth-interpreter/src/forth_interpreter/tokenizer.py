"""Split Forth source into tokens, dropping ``( ... )`` comments."""

from __future__ import annotations

from dataclasses import dataclass

from .errors import CompileError


@dataclass(frozen=True)
class Token:
    """``kind`` is ``"word"``, or ``"string"`` for the text of ``." ..."``."""

    kind: str
    text: str


def tokenize(source: str) -> list[Token]:
    """Tokenize *source*. An unterminated comment or string is a
    :class:`CompileError`."""
    tokens: list[Token] = []
    i = 0
    n = len(source)
    while i < n:
        ch = source[i]
        if ch.isspace():
            i += 1
            continue

        j = i
        while j < n and not source[j].isspace():
            j += 1
        word = source[i:j]

        if word == "(":
            close = source.find(")", j)
            if close == -1:
                raise CompileError("unterminated string/comment")
            i = close + 1
            continue

        if word == '."':
            # One space separates ." from the text; any more are part of it.
            pos = j
            if pos < n and source[pos] == " ":
                pos += 1
            close = source.find('"', pos)
            if close == -1:
                raise CompileError("unterminated string/comment")
            tokens.append(Token("string", source[pos:close]))
            i = close + 1
            continue

        tokens.append(Token("word", word))
        i = j

    return tokens
