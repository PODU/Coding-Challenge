"""Parser unit tests asserting AST shape, precedence and associativity."""

from __future__ import annotations

from decimal import Decimal

import pytest

from calc.ast_nodes import Binary, Call, Const, Num, Unary
from calc.errors import ParseError
from calc.lexer import tokenize
from calc.parser import parse


def ast(source: str):  # type: ignore[no-untyped-def]
    return parse(tokenize(source))


def test_precedence_mul_binds_tighter() -> None:
    tree = ast("1 + 2 * 3")
    assert isinstance(tree, Binary) and tree.op == "+"
    assert isinstance(tree.left, Num) and tree.left.value == Decimal(1)
    assert isinstance(tree.right, Binary) and tree.right.op == "*"


def test_left_associativity_subtraction() -> None:
    # 8 - 3 - 2  ==  (8 - 3) - 2
    tree = ast("8 - 3 - 2")
    assert isinstance(tree, Binary) and tree.op == "-"
    assert isinstance(tree.left, Binary) and tree.left.op == "-"
    assert isinstance(tree.right, Num) and tree.right.value == Decimal(2)


def test_power_right_associativity() -> None:
    # 2 ^ 3 ^ 2  ==  2 ^ (3 ^ 2)
    tree = ast("2 ^ 3 ^ 2")
    assert isinstance(tree, Binary) and tree.op == "^"
    assert isinstance(tree.left, Num) and tree.left.value == Decimal(2)
    assert isinstance(tree.right, Binary) and tree.right.op == "^"


def test_parentheses_override_precedence() -> None:
    tree = ast("(1 + 1) * 5")
    assert isinstance(tree, Binary) and tree.op == "*"
    assert isinstance(tree.left, Binary) and tree.left.op == "+"


def test_unary_minus() -> None:
    tree = ast("-3")
    assert isinstance(tree, Unary) and tree.op == "-"
    assert isinstance(tree.operand, Num)


def test_double_unary() -> None:
    tree = ast("- -5")
    assert isinstance(tree, Unary)
    assert isinstance(tree.operand, Unary)


def test_redundant_nesting() -> None:
    tree = ast("((3))")
    assert isinstance(tree, Num) and tree.value == Decimal(3)


def test_function_call() -> None:
    tree = ast("sin(0)")
    assert isinstance(tree, Call) and tree.name == "sin"
    assert isinstance(tree.arg, Num)


def test_constant() -> None:
    tree = ast("pi")
    assert isinstance(tree, Const) and tree.name == "pi"


@pytest.mark.parametrize(
    "text",
    ["(1 + 2", "()", "1 +", "* 2", "1 2", "", "   ", "sin(1", "1 + )"],
)
def test_syntax_errors_raise_parse_error(text: str) -> None:
    with pytest.raises(ParseError):
        ast(text)


def test_unbalanced_close_paren_has_pos() -> None:
    with pytest.raises(ParseError) as exc:
        ast("1 + 2)")
    assert exc.value.pos == 5
