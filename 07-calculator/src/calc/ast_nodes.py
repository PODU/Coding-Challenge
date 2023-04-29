"""AST node types. Every node keeps ``pos`` so runtime errors can point at a column."""

from __future__ import annotations

from dataclasses import dataclass
from decimal import Decimal
from typing import Union


@dataclass(frozen=True)
class Num:
    value: Decimal
    pos: int = 0


@dataclass(frozen=True)
class Unary:
    """Prefix ``+`` or ``-``."""

    op: str
    operand: "Expr"
    pos: int = 0


@dataclass(frozen=True)
class Binary:
    op: str
    left: "Expr"
    right: "Expr"
    pos: int = 0


@dataclass(frozen=True)
class Call:
    """A function call such as ``sin(0)``."""

    name: str
    arg: "Expr"
    pos: int = 0


@dataclass(frozen=True)
class Const:
    """A named constant such as ``pi``."""

    name: str
    pos: int = 0


Expr = Union[Num, Unary, Binary, Call, Const]
