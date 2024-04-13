"""Food placement."""

from __future__ import annotations

import random
from collections.abc import Iterable

from .model import Point


class FoodSpawner:
    """Puts food on a random empty cell. Pass a seeded ``random.Random`` (or a
    stub) to make placement repeatable."""

    def __init__(self, rng: random.Random) -> None:
        self._rng = rng

    def spawn(
        self, width: int, height: int, occupied: Iterable[Point]
    ) -> Point | None:
        """Return a random empty cell, or ``None`` if the snake fills the board."""
        occupied_set = set(occupied)
        free = [
            Point(x, y)
            for y in range(height)
            for x in range(width)
            if Point(x, y) not in occupied_set
        ]
        if not free:
            return None
        return self._rng.choice(free)
