"""Terminal Snake.

The game rules have no I/O so they can be tested directly; the curses front
end is in :mod:`snake.cli`.
"""

from __future__ import annotations

from .food import FoodSpawner
from .game import SnakeGame
from .model import Direction, GameStatus, Point
from .speed import speed_for

__all__ = [
    "SnakeGame",
    "FoodSpawner",
    "Point",
    "Direction",
    "GameStatus",
    "speed_for",
]

__version__ = "1.0.0"
