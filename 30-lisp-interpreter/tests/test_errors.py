"""Error-handling tests: each asserts the specific Lisp*Error subclass."""

from __future__ import annotations

import pytest

from cclisp.builtins import standard_env
from cclisp.evaluator import evaluate
from cclisp.reader import parse
from cclisp.types import (
    LispArityError,
    LispError,
    LispNameError,
    LispSyntaxError,
    LispTypeError,
)


def ev(src: str, env=None):
    env = env or standard_env()
    return evaluate(parse(src), env)


def test_unbound_symbol():
    with pytest.raises(LispNameError):
        ev("does-not-exist")


def test_unbound_symbol_message():
    with pytest.raises(LispNameError, match="unbound symbol: foo"):
        ev("foo")


def test_extra_close_paren():
    with pytest.raises(LispSyntaxError):
        ev("(+ 1 2))")


def test_missing_close_paren():
    with pytest.raises(LispSyntaxError):
        ev("(+ 1 2")


def test_unterminated_string():
    with pytest.raises(LispSyntaxError):
        ev('"oops')


def test_arity_too_few():
    env = standard_env()
    ev("(defun f (a b) (+ a b))", env)
    with pytest.raises(LispArityError):
        ev("(f 1)", env)


def test_arity_too_many():
    env = standard_env()
    ev("(defun f (a) a)", env)
    with pytest.raises(LispArityError):
        ev("(f 1 2 3)", env)


def test_type_error_arithmetic():
    with pytest.raises(LispTypeError):
        ev('(+ 1 "a")')


def test_type_error_car():
    with pytest.raises(LispTypeError):
        ev("(car 5)")


def test_min_max_abs_reject_non_numbers():
    for src in ("(max t 5)", "(min nil 3)", "(abs t)", '(max 1 "a")'):
        with pytest.raises(LispTypeError):
            ev(src)


def test_division_by_zero():
    with pytest.raises(LispError):
        ev("(/ 1 0)")


def test_set_unbound_raises():
    with pytest.raises(LispNameError):
        ev("(set! never-defined 1)")


def test_call_non_procedure():
    with pytest.raises(LispError):
        ev("(5 1 2)")


def test_lisp_errors_share_base():
    for exc in (LispSyntaxError, LispNameError, LispTypeError, LispArityError):
        assert issubclass(exc, LispError)
