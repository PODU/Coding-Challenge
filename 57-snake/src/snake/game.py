"""Game state and rules. No I/O happens here; randomness comes from the
:class:`FoodSpawner` passed in."""

from __future__ import annotations

import random

from .food import FoodSpawner
from .model import Direction, GameStatus, Point


class SnakeGame:
    """One game on a ``width`` x ``height`` grid. ``snake[0]`` is the head.
    Running into the edge ends the game; there is no wrap-around."""

    def __init__(
        self,
        width: int = 20,
        height: int = 20,
        start: Point | None = None,
        start_length: int = 4,
        direction: Direction = Direction.RIGHT,
        spawner: FoodSpawner | None = None,
    ) -> None:
        if width < 1 or height < 1:
            raise ValueError("board must be at least 1x1")
        if start_length < 1:
            raise ValueError("start_length must be >= 1")

        self._width = width
        self._height = height
        self._start_length = start_length
        self._spawner = spawner if spawner is not None else FoodSpawner(random.Random())

        if start is None:
            start = Point(width // 2, height // 2)
        self._start = start
        self._start_direction = direction

        snake = self._build_snake(start, start_length, direction)
        if not all(self._in_bounds(p) for p in snake):
            raise ValueError(
                "start_length/start does not fit on the board "
                "in the initial direction"
            )

        self._snake: list[Point] = snake
        self._direction = direction
        self._next_direction = direction
        self._score = 0
        self._status = GameStatus.RUNNING
        self._food = self._spawner.spawn(width, height, set(self._snake))

        # reset() puts the food back here so a restart replays the same game.
        self._initial_food = self._food

    @staticmethod
    def _build_snake(
        start: Point, length: int, direction: Direction
    ) -> list[Point]:
        """Head at ``start``, body trailing away from ``direction``."""
        back = direction.opposite.delta
        return [
            Point(start.x + back.x * i, start.y + back.y * i)
            for i in range(length)
        ]

    def _in_bounds(self, p: Point) -> bool:
        return 0 <= p.x < self._width and 0 <= p.y < self._height

    @property
    def width(self) -> int:
        return self._width

    @property
    def height(self) -> int:
        return self._height

    @property
    def snake(self) -> list[Point]:
        return list(self._snake)

    @property
    def head(self) -> Point:
        return self._snake[0]

    @property
    def food(self) -> Point | None:
        return self._food

    @property
    def score(self) -> int:
        return self._score

    @property
    def status(self) -> GameStatus:
        return self._status

    @property
    def direction(self) -> Direction:
        return self._direction

    def set_direction(self, d: Direction) -> None:
        """Queue a turn for the next ``step``.

        Reversing is ignored. The check is against the direction of the last
        step, not the queued one, so two quick key presses within one tick
        can't turn the snake back into itself.
        """
        if self._status is not GameStatus.RUNNING:
            return
        if d is self._direction.opposite:
            return
        self._next_direction = d

    def step(self) -> GameStatus:
        """Advance one tick and return the status. Does nothing once the game is over."""
        if self._status is not GameStatus.RUNNING:
            return self._status

        self._direction = self._next_direction
        new_head = self.head + self._direction.delta

        if not self._in_bounds(new_head):
            self._status = GameStatus.LOST
            return self._status

        growing = new_head == self._food

        # Unless the snake is growing, the tail moves out of the way this tick,
        # so the head is allowed into its cell.
        body = self._snake if growing else self._snake[:-1]
        if new_head in body:
            self._status = GameStatus.LOST
            return self._status

        self._snake.insert(0, new_head)

        if growing:
            self._score += 1
            self._food = self._spawner.spawn(
                self._width, self._height, set(self._snake)
            )
            if self._food is None:
                self._status = GameStatus.WON
        else:
            self._snake.pop()

        return self._status

    def reset(self) -> None:
        """Start the same game over."""
        self._snake = self._build_snake(
            self._start, self._start_length, self._start_direction
        )
        self._direction = self._start_direction
        self._next_direction = self._start_direction
        self._score = 0
        self._status = GameStatus.RUNNING
        self._food = self._initial_food
