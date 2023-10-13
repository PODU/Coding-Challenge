"""Split Lisp source into tokens.

The usual ``replace("(", " ( ").split()`` trick breaks on string literals that
contain spaces, parens or semicolons, so this scans character by character.
"""

from __future__ import annotations

from .types import LispSyntaxError

_DELIMITERS = '()"; \t\r\n'


def tokenize(text: str) -> list[str]:
    """Split *text* into tokens.

    String tokens keep their quotes and escapes; the reader decodes them.
    ``'`` is a token of its own.
    """
    tokens: list[str] = []
    i = 0
    n = len(text)
    while i < n:
        c = text[i]

        if c in " \t\r\n":
            i += 1
            continue

        if c == ";":
            while i < n and text[i] != "\n":
                i += 1
            continue

        if c in "()'":
            tokens.append(c)
            i += 1
            continue

        if c == '"':
            i += 1
            buf = ['"']
            while True:
                if i >= n:
                    raise LispSyntaxError("unterminated string literal")
                ch = text[i]
                if ch == "\\":
                    if i + 1 >= n:
                        raise LispSyntaxError("unterminated string literal")
                    buf.append(ch)
                    buf.append(text[i + 1])
                    i += 2
                    continue
                buf.append(ch)
                i += 1
                if ch == '"':
                    break
            tokens.append("".join(buf))
            continue

        start = i
        while i < n and text[i] not in _DELIMITERS:
            i += 1
        tokens.append(text[start:i])

    return tokens
