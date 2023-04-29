"""Lexer: turns an expression string into tokens ending with EOF."""

from __future__ import annotations

from .errors import LexError
from .tokens import Token, TokenType

_SINGLE: dict[str, TokenType] = {
    "+": TokenType.PLUS,
    "-": TokenType.MINUS,
    "*": TokenType.STAR,
    "/": TokenType.SLASH,
    "^": TokenType.CARET,
    "(": TokenType.LPAREN,
    ")": TokenType.RPAREN,
}


def tokenize(source: str) -> list[Token]:
    tokens: list[Token] = []
    i = 0
    n = len(source)

    while i < n:
        ch = source[i]

        if ch.isspace():
            i += 1
            continue

        if ch in _SINGLE:
            tokens.append(Token(_SINGLE[ch], ch, i))
            i += 1
            continue

        if ch.isdigit() or ch == ".":
            token, i = _scan_number(source, i)
            tokens.append(token)
            continue

        if ch.isalpha() or ch == "_":
            token, i = _scan_ident(source, i)
            tokens.append(token)
            continue

        raise LexError(f"unknown character {ch!r}", pos=i)

    tokens.append(Token(TokenType.EOF, "", n))
    return tokens


def _scan_number(source: str, start: int) -> tuple[Token, int]:
    """Scan ``12``, ``3.14``, ``.5``, ``5.``, ``1e3`` or ``2.5E-4`` at ``start``."""
    n = len(source)
    i = start
    seen_dot = False
    seen_exp = False

    while i < n:
        ch = source[i]
        if ch.isdigit():
            i += 1
        elif ch == ".":
            if seen_dot or seen_exp:
                raise LexError("malformed number", pos=i)
            seen_dot = True
            i += 1
        elif ch in ("e", "E") and not seen_exp:
            # "e" right after a non-digit is an identifier, not an exponent.
            if i == start or not (source[i - 1].isdigit() or source[i - 1] == "."):
                break
            seen_exp = True
            i += 1
            if i < n and source[i] in ("+", "-"):
                i += 1
            if i >= n or not source[i].isdigit():
                raise LexError("malformed number", pos=start)
        else:
            break

    lexeme = source[start:i]

    if lexeme in (".", "") or lexeme.count(".") > 1:
        raise LexError("malformed number", pos=start)

    return Token(TokenType.NUMBER, lexeme, start), i


def _scan_ident(source: str, start: int) -> tuple[Token, int]:
    n = len(source)
    i = start + 1
    while i < n and (source[i].isalnum() or source[i] == "_"):
        i += 1
    return Token(TokenType.IDENT, source[start:i], start), i
