"""Curses front end: drawing, keyboard input and the game loop timer.

The game speeds up as the score rises (see :func:`speed_for`).
"""

from __future__ import annotations

import argparse
import curses
import random
import time

from .food import FoodSpawner
from .game import SnakeGame
from .model import Direction, GameStatus

_KEY_TO_DIR = {
    curses.KEY_UP: Direction.UP,
    curses.KEY_DOWN: Direction.DOWN,
    curses.KEY_LEFT: Direction.LEFT,
    curses.KEY_RIGHT: Direction.RIGHT,
    ord("w"): Direction.UP,
    ord("s"): Direction.DOWN,
    ord("a"): Direction.LEFT,
    ord("d"): Direction.RIGHT,
}


def _draw(stdscr: "curses._CursesWindow", game: SnakeGame) -> None:
    stdscr.erase()
    w, h = game.width, game.height

    stdscr.addstr(0, 0, f"Score: {game.score}    (arrows/WASD move, q quit)")

    top = 1
    stdscr.addstr(top, 0, "+" + "-" * w + "+")
    stdscr.addstr(top + h + 1, 0, "+" + "-" * w + "+")
    for row in range(h):
        stdscr.addstr(top + 1 + row, 0, "|" + " " * w + "|")

    if game.food is not None:
        stdscr.addstr(top + 1 + game.food.y, 1 + game.food.x, "*")

    for i, p in enumerate(game.snake):
        stdscr.addstr(top + 1 + p.y, 1 + p.x, "O" if i == 0 else "o")

    if game.status is not GameStatus.RUNNING:
        msg = "YOU WIN!" if game.status is GameStatus.WON else "GAME OVER"
        stdscr.addstr(top + h + 3, 0, f"{msg}  Final score {game.score}.")
        stdscr.addstr(top + h + 4, 0, "Press r to restart or q to quit.")

    stdscr.refresh()


def _new_game(width: int, height: int, seed: int | None) -> SnakeGame:
    rng = random.Random(seed)
    return SnakeGame(width=width, height=height, spawner=FoodSpawner(rng))


def _loop(stdscr: "curses._CursesWindow", args: argparse.Namespace) -> None:
    from .speed import speed_for

    curses.curs_set(0)
    stdscr.nodelay(True)
    stdscr.keypad(True)

    game = _new_game(args.width, args.height, args.seed)
    last_tick = time.monotonic()

    while True:
        _draw(stdscr, game)

        key = stdscr.getch()
        if key in (ord("q"), ord("Q")):
            return
        if game.status is GameStatus.RUNNING:
            if key in _KEY_TO_DIR:
                game.set_direction(_KEY_TO_DIR[key])
        elif key in (ord("r"), ord("R")):
            game = _new_game(args.width, args.height, args.seed)
            last_tick = time.monotonic()

        now = time.monotonic()
        if game.status is GameStatus.RUNNING and now - last_tick >= speed_for(
            game.score
        ):
            game.step()
            last_tick = now

        time.sleep(0.005)


def main() -> None:
    parser = argparse.ArgumentParser(description="Play Snake in the terminal.")
    parser.add_argument("--width", type=int, default=20)
    parser.add_argument("--height", type=int, default=20)
    parser.add_argument(
        "--seed", type=int, default=None, help="seed the food RNG"
    )
    args = parser.parse_args()
    curses.wrapper(_loop, args)


if __name__ == "__main__":
    main()
