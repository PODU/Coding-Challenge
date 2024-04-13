# Snake — Coding Challenge #57

A classic single-player **Snake** game in Python 3. The implementation splits a
**pure, deterministic game-state core** (no I/O, fully unit-tested headlessly)
from a thin **curses renderer + input layer** driven by a fixed-step game loop
whose tick interval shrinks as the score rises (increasing difficulty).

## Layout

```
57-snake/
├── pyproject.toml          # packaging + console script
├── README.md
├── src/snake/
│   ├── __init__.py         # public core API
│   ├── model.py            # Point, Direction, GameStatus
│   ├── game.py             # SnakeGame — the pure rules
│   ├── food.py             # FoodSpawner (injected RNG)
│   ├── speed.py            # speed_for(score)
│   ├── cli.py              # curses renderer + input + loop (impure)
│   └── __main__.py         # python -m snake
└── tests/
    └── test_game.py        # headless core tests
```

## Install

```bash
python -m pip install -e .          # installs the `snake` console script
python -m pip install -e .[dev]     # + pytest
```

The **core** is standard-library only. The renderer uses `curses`, which ships
with CPython on Linux/macOS; on **Windows** the install pulls in
`windows-curses` automatically.

## Play

```bash
snake                 # 20x20 board (after install)
python -m snake       # same, without installing the console script
snake --width 30 --height 20 --seed 42
```

Controls: **arrow keys** or **WASD** to steer, **q** to quit. After a game over
or win, press **r** to restart. The head is drawn `O`, the body `o`, and food
`*`. Speed increases as your score climbs.

## Test

The core is fully deterministic — food randomness is injected, so every rule is
reproducible without a screen, clock, or keyboard.

```bash
python -m pytest
```

Coverage: initial state, movement (head-add/tail-drop), eat + grow + score,
food never on the snake (seeded RNG), all four wall collisions, self-collision,
the tail-follow edge case, the reverse-direction guard (including the
multi-key-per-tick bypass), reset/restart, board-full win, the speed curve, and
no-ops after game over.

## Design notes

- **Deterministic core:** all randomness is an injected `random.Random` (or a
  scripted stub), so rules are unit-testable headlessly.
- **No rules in the renderer:** `cli.py` only draws state and forwards keys.
- **Reverse guard:** a turn is rejected if it is the direct opposite of the
  *committed* heading, so rapid multi-key presses within one tick cannot bypass
  it and reverse into the neck.
- **Walls are lethal** (no wrap), per the challenge.
- **Win when full:** if eating fills the last empty cell, `FoodSpawner.spawn`
  returns `None` and the game ends `WON`.
