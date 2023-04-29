"""Evaluator unit tests: edge-case table and error types."""

from __future__ import annotations

from decimal import Decimal

import pytest

from calc.api import evaluate, format_result
from calc.errors import EvalError, LexError, ParseError


@pytest.mark.parametrize(
    "expr,expected",
    [
        # Division
        ("7 / 2", "3.5"),
        # Float precision via Decimal
        ("0.1 + 0.2", "0.3"),
        # Unary / negatives
        ("-5", "-5"),
        ("- -5", "5"),
        ("3 * -2", "-6"),
        ("-(2 + 3)", "-5"),
        ("-3 + 4", "1"),
        # Precedence / associativity
        ("1 + 1 * 5", "6"),
        ("2 ^ 3 ^ 2", "512"),
        ("8 - 3 - 2", "3"),
        # Parentheses
        ("((3))", "3"),
        # Whitespace
        ("  1+1*5 ", "6"),
        # Numbers
        (".5", "0.5"),
        ("5.", "5"),
        ("3.14", "3.14"),
        # Functions / constants
        ("sin(0)", "0"),
        ("cos(0)", "1"),
        ("sqrt(9)", "3"),
        ("abs(-4)", "4"),
    ],
)
def test_eval_values(expr: str, expected: str) -> None:
    assert format_result(evaluate(expr)) == expected


def test_constants_pi_and_e() -> None:
    assert str(evaluate("pi")).startswith("3.14159")
    assert str(evaluate("e")).startswith("2.71828")


def test_division_by_zero() -> None:
    with pytest.raises(EvalError):
        evaluate("1 / 0")


def test_unknown_function() -> None:
    with pytest.raises(EvalError) as exc:
        evaluate("foo(2)")
    assert "unknown function" in exc.value.message


def test_unknown_constant() -> None:
    with pytest.raises(EvalError):
        evaluate("bar")


def test_sqrt_negative_domain_error() -> None:
    with pytest.raises(EvalError):
        evaluate("sqrt(-1)")


def test_overflow() -> None:
    with pytest.raises(EvalError) as exc:
        evaluate("10 ^ 10000000")
    assert "overflow" in exc.value.message


@pytest.mark.parametrize("expr", ["1 + @", "1.2.3"])
def test_lex_errors(expr: str) -> None:
    with pytest.raises(LexError):
        evaluate(expr)


@pytest.mark.parametrize("expr", ["(1 + 2", "()", "1 +", "* 2", "1 2"])
def test_parse_errors(expr: str) -> None:
    with pytest.raises(ParseError):
        evaluate(expr)


def test_unary_plus_after_binary_is_valid() -> None:
    # The grammar (unary := ("+"|"-") unary) and its reference evaluator
    # (Python eval) both treat "1 + + 2" as 1 + (+2) = 3. We honour the grammar.
    assert evaluate("1 + + 2") == Decimal(3)


def test_one_third_precision() -> None:
    # 1/3 is truncated to the Decimal context precision, not silently wrong.
    result = evaluate("1 / 3")
    assert str(result).startswith("0.3333333333")


@pytest.mark.parametrize(
    "value,precision,expected",
    [
        (Decimal("4.0"), None, "4"),
        (Decimal("1.50"), None, "1.5"),
        (Decimal("0"), None, "0"),
        (Decimal("-0"), None, "0"),
        (Decimal("3.14159265"), 3, "3.14"),
    ],
)
def test_format_result(value: Decimal, precision: int | None, expected: str) -> None:
    assert format_result(value, precision) == expected
