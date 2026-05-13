"""Policies map a preprocessed observation to a discrete action.

    - RandomPolicy        : uniform baseline.
    - ReinforcePolicy     : wraps a PolicyNet; samples (train) / argmax (eval).
    - ScriptedPaddlePolicy: deterministic ball-tracking heuristic, test-only.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Protocol, runtime_checkable

import numpy as np

from .engine import EngineState, predict_intercept
from .network import PolicyNet, softmax

# Minimal Atari Pong action set mapped onto the network's 3-way head.
# ALE Pong raw actions: 0=NOOP, 2=RIGHT(=UP), 3=LEFT(=DOWN).
PONG_ACTIONS: tuple[int, ...] = (0, 2, 3)
ACTION_MEANINGS: tuple[str, ...] = ("NOOP", "UP", "DOWN")


@dataclass
class ActionResult:
    """``action`` indexes the action set; ``log_prob`` is None for policies
    without a network."""

    action: int
    log_prob: float | None = None


@runtime_checkable
class Policy(Protocol):
    def act(self, observation: np.ndarray, *, greedy: bool = False) -> ActionResult:
        ...


class RandomPolicy:
    """Uniform random baseline. ``greedy`` is ignored."""

    def __init__(self, n_actions: int = len(PONG_ACTIONS), seed: int = 42) -> None:
        self.n_actions = n_actions
        self._rng = np.random.default_rng(seed)

    def act(self, observation: np.ndarray, *, greedy: bool = False) -> ActionResult:
        return ActionResult(action=int(self._rng.integers(self.n_actions)))


class ReinforcePolicy:
    """Samples from a :class:`PolicyNet` (seeded), or takes the argmax if ``greedy``."""

    def __init__(self, net: PolicyNet, seed: int = 42) -> None:
        self.net = net
        self._rng = np.random.default_rng(seed)

    def act(self, observation: np.ndarray, *, greedy: bool = False) -> ActionResult:
        probs = softmax(self.net.forward(observation))[0]
        if greedy:
            action = int(np.argmax(probs))
        else:
            action = int(self._rng.choice(len(probs), p=probs))
        log_prob = float(np.log(np.clip(probs[action], 1e-12, 1.0)))
        return ActionResult(action=action, log_prob=log_prob)


class ScriptedPaddlePolicy:
    """Ball-tracking right paddle for the test engine.

    Reads engine state rather than pixels, predicts where the ball will arrive
    and moves toward it at no more than ``max_speed`` so it can still be beaten.
    Use ``move``; ``act`` is not supported.
    """

    def __init__(
        self,
        paddle_x: float,
        field_height: float,
        max_speed: float = 1.0,
    ) -> None:
        self.paddle_x = paddle_x
        self.field_height = field_height
        self.max_speed = max_speed

    def move(self, state: "EngineState") -> float:
        """Return this tick's paddle movement, within ``[-max_speed, max_speed]``."""
        target = predict_intercept(
            ball_x=state.ball_x,
            ball_y=state.ball_y,
            vx=state.vx,
            vy=state.vy,
            paddle_x=self.paddle_x,
            field_height=self.field_height,
            fallback_y=state.right_y,
        )
        return float(np.clip(target - state.right_y, -self.max_speed, self.max_speed))

    def act(self, observation: np.ndarray, *, greedy: bool = False) -> ActionResult:
        raise NotImplementedError(
            "ScriptedPaddlePolicy reads engine state via .move(); it does not "
            "act on pixel observations."
        )
