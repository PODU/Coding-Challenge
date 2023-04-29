"""Tokenizer unit tests."""

from __future__ import annotations

import pytest

from calc.errors import LexError
from calc.lexer import tokenize
from calc.tokens import TokenType


def _types(source: str) -> list[TokenType]:
    return [t.type for t in tokenize(source)]


def test_single_char_tokens() -> None:
    assert _types("+-*/^()") == [
        TokenType.PLUS,
        TokenType.MINUS,
        TokenType.STAR,
        TokenType.SLASH,
        TokenType.CARET,
        TokenType.LPAREN,
        TokenType.RPAREN,
        TokenType.EOF,
    ]


def test_whitespace_is_skipped() -> None:
    assert _types("  1\t+\n2 ") == [
        TokenType.NUMBER,
        TokenType.PLUS,
        TokenType.NUMBER,
        TokenType.EOF,
    ]


@pytest.mark.parametrize("text", ["12", "3.14", ".5", "5.", "0", "1e3", "2.5E-4"])
def test_number_forms(text: str) -> None:
    toks = tokenize(text)
    assert toks[0].type is TokenType.NUMBER
    assert toks[0].lexeme == text


def test_identifier() -> None:
    toks = tokenize("sin")
    assert toks[0].type is TokenType.IDENT
    assert toks[0].lexeme == "sin"


def test_token_positions() -> None:
    toks = tokenize("1 + 22")
    assert toks[0].pos == 0
    assert toks[1].pos == 2
    assert toks[2].pos == 4


@pytest.mark.parametrize("text,col", [("1 + @", 4), ("1 $ 2", 2), ("#", 0)])
def test_unknown_character_raises_with_pos(text: str, col: int) -> None:
    with pytest.raises(LexError) as exc:
        tokenize(text)
    assert exc.value.pos == col


@pytest.mark.parametrize("text", ["1.2.3", ".", "1e", "1e+"])
def test_malformed_number_raises(text: str) -> None:
    with pytest.raises(LexError):
        tokenize(text)
