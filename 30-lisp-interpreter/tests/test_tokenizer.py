"""Tokenizer tests."""

from __future__ import annotations

import pytest

from cclisp.tokenizer import tokenize
from cclisp.types import LispSyntaxError


def test_simple_expression():
    assert tokenize("(+ 1 2)") == ["(", "+", "1", "2", ")"]


def test_nested_expression():
    assert tokenize("(defun doublen (n) (* n 2))") == [
        "(", "defun", "doublen", "(", "n", ")", "(", "*", "n", "2", ")", ")",
    ]


def test_parens_without_spaces():
    assert tokenize("(a(b)c)") == ["(", "a", "(", "b", ")", "c", ")"]


def test_string_with_spaces_and_parens_kept_intact():
    toks = tokenize('(format t "a (b) c")')
    assert toks == ["(", "format", "t", '"a (b) c"', ")"]


def test_string_with_escaped_quote():
    toks = tokenize(r'"he said \"hi\""')
    assert toks == [r'"he said \"hi\""']


def test_line_comment_is_stripped():
    assert tokenize("(+ 1 2) ; trailing\n(+ 3 4)") == [
        "(", "+", "1", "2", ")", "(", "+", "3", "4", ")",
    ]


def test_quote_reader_macro_token():
    assert tokenize("'(1 2)") == ["'", "(", "1", "2", ")"]


def test_whitespace_variants():
    assert tokenize("\t( +\n1\r2 )\n") == ["(", "+", "1", "2", ")"]


def test_empty_input():
    assert tokenize("   \n  ; only a comment\n") == []


def test_unterminated_string_raises():
    with pytest.raises(LispSyntaxError):
        tokenize('"no closing quote')
