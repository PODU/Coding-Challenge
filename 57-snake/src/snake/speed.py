"""How fast the game runs at a given score."""

from __future__ import annotations


def speed_for(
    score: int,
    base: float = 0.18,
    minimum: float = 0.06,
    step: float = 0.005,
) -> float:
    """Seconds between ticks at ``score``. Shrinks by ``step`` per point,
    down to ``minimum``."""
    return max(minimum, base - score * step)
