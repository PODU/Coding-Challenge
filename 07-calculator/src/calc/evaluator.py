"""Tree-walking evaluator over Decimal. Runtime problems raise EvalError."""

from __future__ import annotations

import decimal
import math
from decimal import Decimal
from typing import Callable

from .ast_nodes import Binary, Call, Const, Expr, Num, Unary
from .errors import EvalError


def _dec_sin(x: Decimal) -> Decimal:
    return Decimal(str(math.sin(float(x))))


def _dec_cos(x: Decimal) -> Decimal:
    return Decimal(str(math.cos(float(x))))


def _dec_tan(x: Decimal) -> Decimal:
    return Decimal(str(math.tan(float(x))))


def _dec_sqrt(x: Decimal) -> Decimal:
    if x < 0:
        raise EvalError("domain error: sqrt of negative number")
    return x.sqrt()


FUNCTIONS: dict[str, Callable[[Decimal], Decimal]] = {
    "sin": _dec_sin,
    "cos": _dec_cos,
    "tan": _dec_tan,
    "sqrt": _dec_sqrt,
    "abs": abs,
}


CONSTANTS: dict[str, Decimal] = {
    "pi": Decimal(str(math.pi)),
    "e": Decimal(str(math.e)),
}


def evaluate_ast(node: Expr) -> Decimal:
    match node:
        case Num(value):
            return value

        case Const(name):
            try:
                return CONSTANTS[name]
            except KeyError:
                raise EvalError(
                    f"unknown constant {name!r}", pos=node.pos
                ) from None

        case Call(name, arg):
            func = FUNCTIONS.get(name)
            if func is None:
                raise EvalError(f"unknown function {name!r}", pos=node.pos)
            value = evaluate_ast(arg)
            try:
                return func(value)
            except EvalError:
                raise
            except (ValueError, OverflowError, decimal.DecimalException) as exc:
                raise EvalError(
                    f"domain error in {name!r}", pos=node.pos
                ) from exc

        case Unary(op, operand):
            value = evaluate_ast(operand)
            return -value if op == "-" else +value

        case Binary(op, left, right):
            return _eval_binary(node, op, evaluate_ast(left), evaluate_ast(right))

    raise EvalError(f"cannot evaluate node {node!r}")  # pragma: no cover


def _eval_binary(node: Binary, op: str, left: Decimal, right: Decimal) -> Decimal:
    try:
        if op == "+":
            return left + right
        if op == "-":
            return left - right
        if op == "*":
            return left * right
        if op == "/":
            if right == 0:
                raise EvalError("division by zero", pos=node.pos)
            return left / right
        if op == "^":
            return _power(node, left, right)
    except decimal.DivisionByZero as exc:
        raise EvalError("division by zero", pos=node.pos) from exc
    except decimal.Overflow as exc:
        raise EvalError("overflow", pos=node.pos) from exc
    except decimal.InvalidOperation as exc:
        raise EvalError("invalid operation", pos=node.pos) from exc

    raise EvalError(f"unknown operator {op!r}", pos=node.pos)  # pragma: no cover


def _power(node: Binary, base: Decimal, exponent: Decimal) -> Decimal:
    # Huge integer exponents would make Decimal grind for a very long time
    # before overflowing, so reject them up front.
    if exponent == exponent.to_integral_value():
        if abs(exponent) > Decimal("1000000"):
            raise EvalError("overflow", pos=node.pos)
        try:
            return base ** exponent
        except decimal.Overflow as exc:
            raise EvalError("overflow", pos=node.pos) from exc

    if base < 0:
        raise EvalError("domain error: negative base to fractional power", pos=node.pos)
    try:
        return base ** exponent
    except decimal.Overflow as exc:
        raise EvalError("overflow", pos=node.pos) from exc
    except decimal.InvalidOperation as exc:
        raise EvalError("domain error", pos=node.pos) from exc
