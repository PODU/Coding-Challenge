"""Headless unit tests for the pure Snake core.

No curses, no sleep, no real RNG: food is made deterministic by injecting a
seeded ``random.Random`` or a scripted stub spawner.
"""

from __future__ import annotations

import random

import pytest

from snake import (
    Direction,
    FoodSpawner,
    GameStatus,
    Point,
    SnakeGame,
    speed_for,
)


class ScriptedSpawner:
    """A :class:`FoodSpawner`-compatible stub returning scripted points."""

    def __init__(self, points: list[Point | None]) -> None:
        self._points = list(points)

    def spawn(self, width: int, height: int, occupied) -> Point | None:
        if self._points:
            return self._points.pop(0)
        return Point(width - 1, height - 1)


def far_food_game(**kwargs) -> SnakeGame:
    """A game whose food sits in a corner, out of the way of scripted paths."""
    spawner = ScriptedSpawner([Point(0, 0)] * 50)
    return SnakeGame(spawner=spawner, **kwargs)


# initial state
def test_initial_state() -> None:
    game = SnakeGame(spawner=FoodSpawner(random.Random(1)))
    assert len(game.snake) == 4
    assert game.head == game.snake[0]
    assert game.head not in game.snake[1:]  # head distinct
    assert game.score == 0
    assert game.status is GameStatus.RUNNING
    assert game.food is not None
    assert game.food not in game.snake  # food off-snake
    assert game.direction is Direction.RIGHT


# movement
def test_move_advances_head_drops_tail() -> None:
    game = far_food_game(start=Point(10, 10), start_length=4)
    old_tail = game.snake[-1]
    status = game.step()
    assert status is GameStatus.RUNNING
    assert game.head == Point(11, 10)  # advanced in heading
    assert len(game.snake) == 4  # length unchanged
    assert old_tail not in game.snake  # tail dropped


# eat & grow
def test_eat_grows_and_scores() -> None:
    # Initial food in front of the head; second spawn parked off-snake.
    spawner = ScriptedSpawner([Point(11, 10), Point(0, 0)])
    game = SnakeGame(spawner=spawner, start=Point(10, 10), start_length=4)
    assert game.food == Point(11, 10)

    game.step()

    assert game.score == 1
    assert len(game.snake) == 5  # grew (tail kept)
    assert game.head == Point(11, 10)
    assert game.food == Point(0, 0)
    assert game.food not in game.snake  # new food off-snake


# food placement
def test_food_never_on_snake() -> None:
    rng = random.Random(12345)
    spawner = FoodSpawner(rng)
    occupied = {Point(x, 5) for x in range(10)}
    for _ in range(2000):
        p = spawner.spawn(20, 20, occupied)
        assert p is not None
        assert p not in occupied


def test_food_full_board_returns_none() -> None:
    spawner = FoodSpawner(random.Random(0))
    occupied = {Point(x, y) for x in range(2) for y in range(2)}
    assert spawner.spawn(2, 2, occupied) is None


# wall collisions
@pytest.mark.parametrize(
    "start, direction",
    [
        (Point(0, 5), Direction.LEFT),
        (Point(19, 5), Direction.RIGHT),
        (Point(5, 0), Direction.UP),
        (Point(5, 19), Direction.DOWN),
    ],
)
def test_wall_collision(start: Point, direction: Direction) -> None:
    game = far_food_game(start=start, start_length=1, direction=direction)
    assert game.step() is GameStatus.LOST
    assert game.status is GameStatus.LOST


# self collision and the tail-follow edge case
def test_self_collision() -> None:
    game = far_food_game(start=Point(10, 10), start_length=5)
    game.set_direction(Direction.DOWN)
    assert game.step() is GameStatus.RUNNING
    game.set_direction(Direction.LEFT)
    assert game.step() is GameStatus.RUNNING
    game.set_direction(Direction.UP)
    assert game.step() is GameStatus.LOST  # head crosses its own body


def test_tail_cell_is_not_self_collision() -> None:
    game = far_food_game(start=Point(3, 3), start_length=4)
    game.set_direction(Direction.DOWN)
    game.step()
    game.set_direction(Direction.LEFT)
    game.step()
    game.set_direction(Direction.UP)
    # The head moves into the cell the tail just vacated -> still legal.
    assert game.step() is GameStatus.RUNNING


# reverse-direction guard
def test_reverse_direction_ignored() -> None:
    game = far_food_game(start=Point(10, 10), start_length=4)
    game.set_direction(Direction.LEFT)  # direct reverse of RIGHT
    assert game.direction is Direction.RIGHT
    game.step()
    assert game.head == Point(11, 10)  # still moving right, not into neck


def test_reverse_guard_not_bypassed_by_double_press() -> None:
    # From RIGHT: a perpendicular turn then the reverse within one tick must
    # not let the snake reverse into its neck. The reverse is checked against
    # the committed heading, so it stays rejected.
    game = far_food_game(start=Point(10, 10), start_length=4)
    game.set_direction(Direction.UP)
    game.set_direction(Direction.LEFT)  # opposite of committed RIGHT -> ignored
    game.step()
    assert game.head == Point(10, 9)  # moved UP, never reversed


# reset
def test_reset_restores_initial() -> None:
    spawner = ScriptedSpawner([Point(11, 10), Point(0, 0), Point(1, 1)])
    game = SnakeGame(spawner=spawner, start=Point(10, 10), start_length=4)
    initial_snake = game.snake
    initial_food = game.food

    game.set_direction(Direction.DOWN)
    game.step()
    game.step()

    game.reset()
    assert game.snake == initial_snake
    assert game.food == initial_food
    assert game.score == 0
    assert game.direction is Direction.RIGHT
    assert game.status is GameStatus.RUNNING


# win when board full
def test_win_when_board_full() -> None:
    # 2x1 board, snake length 1: the only empty cell holds the food; eating it
    # fills the board and wins.
    game = SnakeGame(
        width=2,
        height=1,
        start=Point(0, 0),
        start_length=1,
        direction=Direction.RIGHT,
        spawner=FoodSpawner(random.Random(0)),
    )
    assert game.food == Point(1, 0)
    assert game.step() is GameStatus.WON
    assert game.score == 1
    assert game.food is None


# speed
def test_speed_for_decreases_and_clamps() -> None:
    speeds = [speed_for(s) for s in range(0, 60)]
    assert all(b <= a for a, b in zip(speeds, speeds[1:]))  # non-increasing
    assert speed_for(0) == pytest.approx(0.18)
    assert speed_for(10_000) == pytest.approx(0.06)  # clamped at minimum


# no-op after game over
def test_no_op_after_game_over() -> None:
    game = far_food_game(start=Point(0, 5), start_length=1, direction=Direction.LEFT)
    assert game.step() is GameStatus.LOST
    snake_before = game.snake
    game.set_direction(Direction.UP)  # ignored
    assert game.step() is GameStatus.LOST  # no-op
    assert game.snake == snake_before
    assert game.direction is Direction.LEFT


# validation
def test_start_length_off_board_raises() -> None:
    with pytest.raises(ValueError):
        SnakeGame(width=3, height=3, start=Point(1, 1), start_length=5)
