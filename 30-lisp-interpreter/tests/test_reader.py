"""Reader / parser tests."""

from __future__ import annotations

import pytest

from cclisp.reader import atom, parse, parse_all
from cclisp.types import LispSyntaxError, Symbol


def test_r2_exact_ast():
    # The exact example from the design / challenge (R2).
    assert parse("(defun doublen (n) (* n 2))") == [
        "defun", "doublen", ["n"], ["*", "n", 2],
    ]


def test_atom_int():
    v = atom("42")
    assert v == 42 and isinstance(v, int)


def test_atom_negative_int():
    assert atom("-7") == -7


def test_atom_float():
    v = atom("3.14")
    assert v == 3.14 and isinstance(v, float)


def test_atom_string():
    v = atom('"hello world"')
    assert v == "hello world" and isinstance(v, str) and not isinstance(v, Symbol)


def test_atom_string_escapes():
    assert atom('"a\\nb\\tc"') == "a\nb\tc"


def test_atom_symbol():
    v = atom("+")
    assert v == "+" and isinstance(v, Symbol)


def test_atom_booleans():
    assert atom("t") is True
    assert atom("nil") is False


def test_quote_reader_macro_expands():
    assert parse("'x") == ["quote", "x"]
    assert parse("'(1 2 3)") == ["quote", [1, 2, 3]]


def test_parse_all_multiple_forms():
    forms = parse_all("(define x 1) (define y 2)")
    assert forms == [["define", "x", 1], ["define", "y", 2]]


def test_unbalanced_extra_close_paren():
    with pytest.raises(LispSyntaxError):
        parse("(+ 1 2))")


def test_unbalanced_missing_close_paren():
    with pytest.raises(LispSyntaxError):
        parse("(+ 1 2")


def test_lone_close_paren():
    with pytest.raises(LispSyntaxError):
        parse(")")


def test_empty_program_raises():
    with pytest.raises(LispSyntaxError):
        parse("   ")
