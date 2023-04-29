"""Golden / challenge acceptance tests through the public ``evaluate``."""

from __future__ import annotations

import pytest

from calc.api import evaluate, format_result

GOLDEN = [
    ("1 + 2", "3"),
    ("2 - 1", "1"),
    ("2 * 3", "6"),
    ("3 / 2", "1.5"),
    ("1 + 1 * 5", "6"),
    ("(1 + 1) * 5", "10"),
    ("10 / (6 - 1)", "2"),
    ("(1 * 2) - (3 * 4)", "-10"),
]


@pytest.mark.parametrize("expr,expected", GOLDEN)
def test_challenge_golden_cases(expr: str, expected: str) -> None:
    assert format_result(evaluate(expr)) == expected
