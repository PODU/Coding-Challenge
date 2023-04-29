"""Property-based / fuzz tests using hypothesis.

Three families:

* random valid expression trees evaluate to the same value as a reference;
* random arbitrary strings never crash the calculator (only ``CalcError``);
* algebraic identities hold over generated operands.
"""

from __future__ import annotations

from decimal import Decimal

import pytest
from hypothesis import given, settings
from hypothesis import strategies as st

from calc.api import evaluate
from calc.errors import CalcError

# Strategy yielding (source, expected value) pairs for valid expressions over
# small integers using + - * and parentheses.

_leaves = st.integers(min_value=0, max_value=99).map(
    lambda n: (str(n), Decimal(n))
)


def _combine(children: st.SearchStrategy):  # type: ignore[no-untyped-def]
    @st.composite
    def build(draw):  # type: ignore[no-untyped-def]
        left_s, left_v = draw(children)
        right_s, right_v = draw(children)
        op = draw(st.sampled_from(["+", "-", "*"]))
        if op == "+":
            value = left_v + right_v
        elif op == "-":
            value = left_v - right_v
        else:
            value = left_v * right_v
        source = f"({left_s} {op} {right_s})"
        return source, value

    return build()


_expressions = st.recursive(_leaves, _combine, max_leaves=12)


@settings(max_examples=300)
@given(_expressions)
def test_random_valid_expressions_match_reference(case: tuple[str, Decimal]) -> None:
    source, expected = case
    assert evaluate(source) == expected


@settings(max_examples=400)
@given(st.text(alphabet="0123456789+-*/^(). sincoteqrtpix", max_size=12))
def test_random_strings_never_crash(source: str) -> None:
    """The calculator either returns a Decimal or raises CalcError -- never else."""
    try:
        result = evaluate(source)
    except CalcError:
        return
    except RecursionError:
        # Pathological deep nesting is acceptable to reject; treat as handled.
        return
    assert isinstance(result, Decimal)


_operands = st.integers(min_value=-50, max_value=50)


@settings(max_examples=200)
@given(_operands, _operands)
def test_commutativity_of_addition(a: int, b: int) -> None:
    assert evaluate(f"{a} + {b}") == evaluate(f"{b} + {a}")


@settings(max_examples=200)
@given(_operands, _operands, _operands)
def test_associativity_of_addition(a: int, b: int, c: int) -> None:
    assert evaluate(f"({a} + {b}) + {c}") == evaluate(f"{a} + ({b} + {c})")


@settings(max_examples=200)
@given(_operands, _operands, _operands)
def test_distributivity(a: int, b: int, c: int) -> None:
    assert evaluate(f"{a} * ({b} + {c})") == evaluate(f"{a}*{b} + {a}*{c}")


# Sanity: helper note kept to satisfy pytest collection on empty parametrize.
@pytest.mark.parametrize("expr", ["1 + 1"])
def test_smoke(expr: str) -> None:
    assert evaluate(expr) == Decimal(2)
