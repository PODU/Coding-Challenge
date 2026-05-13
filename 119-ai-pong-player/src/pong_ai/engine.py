"""A small deterministic Pong engine and ball-trajectory prediction.

This is not the Atari emulator. It exists so tests can check seeded
reproducibility, the bounce prediction maths and that the scripted paddle can
be beaten.
"""

from __future__ import annotations

from dataclasses import dataclass

import numpy as np


def predict_intercept(
    ball_x: float,
    ball_y: float,
    vx: float,
    vy: float,
    paddle_x: float,
    field_height: float,
    fallback_y: float = 0.0,
) -> float:
    """Predict the ball's y when it reaches ``paddle_x``, including wall bounces.

    The straight-line landing point is folded back into ``[0, field_height]``,
    which handles any number of bounces. Returns ``fallback_y`` if the ball is
    not moving toward the paddle.
    """
    if vx == 0:
        return fallback_y
    t = (paddle_x - ball_x) / vx
    if t < 0:
        return fallback_y
    return _fold(ball_y + vy * t, field_height)


def _fold(y: float, height: float) -> float:
    """Reflect ``y`` into ``[0, height]`` as if bouncing between two walls."""
    if height <= 0:
        return 0.0
    period = 2.0 * height
    m = y % period
    if m < 0:
        m += period
    if m > height:
        m = period - m
    return m


@dataclass
class EngineState:

    ball_x: float
    ball_y: float
    vx: float
    vy: float
    left_y: float
    right_y: float
    left_score: int
    right_score: int

    def as_tuple(self) -> tuple[float, ...]:
        return (
            self.ball_x,
            self.ball_y,
            self.vx,
            self.vy,
            self.left_y,
            self.right_y,
            float(self.left_score),
            float(self.right_score),
        )


class PongEngine:
    """Minimal deterministic Pong.

    ``x`` runs from the left paddle (0) to the right paddle (``width``) and
    ``y`` from 0 to ``height``. A paddle covers ``paddle_half`` either side of
    its centre.
    """

    def __init__(
        self,
        width: float = 40.0,
        height: float = 20.0,
        paddle_half: float = 2.0,
        ball_speed_x: float = 1.0,
        left_spin_vy_max: float = 0.0,
        seed: int = 0,
    ) -> None:
        self.width = width
        self.height = height
        self.paddle_half = paddle_half
        self.ball_speed_x = ball_speed_x
        # When > 0 the left paddle gives the ball a random (seeded) vertical
        # speed on every return. Without it rallies settle into a loop and the
        # scripted paddle never misses.
        self.left_spin_vy_max = left_spin_vy_max
        self._rng = np.random.default_rng(seed)
        self.left_score = 0
        self.right_score = 0
        self.reset()

    def reset(self, vy: float | None = None, toward_right: bool = True) -> None:
        """Serve the ball from the centre."""
        self.ball_x = self.width / 2.0
        self.ball_y = self.height / 2.0
        self.vx = self.ball_speed_x if toward_right else -self.ball_speed_x
        if vy is None:
            vy = float(self._rng.uniform(-1.0, 1.0))
        self.vy = vy
        self.left_y = self.height / 2.0
        self.right_y = self.height / 2.0

    @property
    def state(self) -> EngineState:
        return EngineState(
            ball_x=self.ball_x,
            ball_y=self.ball_y,
            vx=self.vx,
            vy=self.vy,
            left_y=self.left_y,
            right_y=self.right_y,
            left_score=self.left_score,
            right_score=self.right_score,
        )

    def step(
        self, left_move: float, right_move: float
    ) -> tuple[EngineState, int]:
        """Move the paddles by the given amounts and advance the ball one tick.

        Returns ``(state, scored)``: +1 if the right player won a point, -1 if
        the left player did, else 0.
        """
        self.left_y = float(np.clip(self.left_y + left_move, 0.0, self.height))
        self.right_y = float(np.clip(self.right_y + right_move, 0.0, self.height))

        self.ball_x += self.vx
        self.ball_y += self.vy

        if self.ball_y < 0.0:
            self.ball_y = -self.ball_y
            self.vy = -self.vy
        elif self.ball_y > self.height:
            self.ball_y = 2.0 * self.height - self.ball_y
            self.vy = -self.vy

        scored = 0
        if self.ball_x >= self.width:
            if abs(self.ball_y - self.right_y) <= self.paddle_half:
                self.ball_x = 2.0 * self.width - self.ball_x
                self.vx = -self.vx
            else:
                self.left_score += 1
                scored = -1
        elif self.ball_x <= 0.0:
            if abs(self.ball_y - self.left_y) <= self.paddle_half:
                self.ball_x = -self.ball_x
                self.vx = -self.vx
                if self.left_spin_vy_max > 0.0:
                    self.vy = float(
                        self._rng.uniform(
                            -self.left_spin_vy_max, self.left_spin_vy_max
                        )
                    )
            else:
                self.right_score += 1
                scored = 1

        return self.state, scored
