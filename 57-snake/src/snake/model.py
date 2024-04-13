"""Grid points, directions and game status."""

from __future__ import annotations

from dataclasses import dataclass
from enum import Enum


@dataclass(frozen=True)
class Point:
    """A cell: column ``x``, row ``y``."""

    x: int
    y: int

    def __add__(self, other: "Point") -> "Point":
        return Point(self.x + other.x, self.y + other.y)


class Direction(Enum):
    """A heading. The value is the ``(dx, dy)`` of one step."""

    UP = (0, -1)
    DOWN = (0, 1)
    LEFT = (-1, 0)
    RIGHT = (1, 0)

    @property
    def dx(self) -> int:
        return self.value[0]

    @property
    def dy(self) -> int:
        return self.value[1]

    @property
    def delta(self) -> Point:
        return Point(self.value[0], self.value[1])

    @property
    def opposite(self) -> "Direction":
        return _OPPOSITES[self]


_OPPOSITES: dict[Direction, Direction] = {
    Direction.UP: Direction.DOWN,
    Direction.DOWN: Direction.UP,
    Direction.LEFT: Direction.RIGHT,
    Direction.RIGHT: Direction.LEFT,
}


class GameStatus(Enum):
    RUNNING = "running"
    WON = "won"
    LOST = "lost"
