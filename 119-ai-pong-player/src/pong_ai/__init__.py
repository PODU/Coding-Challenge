"""pong_ai: a REINFORCE agent that learns Atari Pong from pixels.

NumPy-only modules (always importable):
    - preprocess : crop / grayscale / resize / flatten / frame-diff
    - returns    : discounted returns (+ point-boundary reset) and normalization
    - network    : PolicyNet (1 hidden layer) forward + REINFORCE backward
    - policy     : Policy interface, RandomPolicy, ReinforcePolicy
    - agent      : RolloutBuffer + reinforce_update
    - engine     : tiny deterministic Pong engine + trajectory prediction
    - checkpoint : save / load model checkpoints (npz)

Modules that need the `atari` extra:
    - env        : make_pong_env (Gymnasium / ALE-py)
    - train      : training entry point
    - evaluate   : greedy eval, video record, reward-curve plot
"""

from __future__ import annotations

from .agent import RolloutBuffer, reinforce_update
from .checkpoint import load_checkpoint, save_checkpoint
from .engine import PongEngine, predict_intercept
from .network import Adam, PolicyNet, softmax
from .policy import (
    ACTION_MEANINGS,
    PONG_ACTIONS,
    ActionResult,
    Policy,
    RandomPolicy,
    ReinforcePolicy,
    ScriptedPaddlePolicy,
)
from .preprocess import frame_diff, preprocess_frame
from .returns import discounted_returns, normalize

__all__ = [
    "ACTION_MEANINGS",
    "ActionResult",
    "Adam",
    "PONG_ACTIONS",
    "Policy",
    "PolicyNet",
    "PongEngine",
    "RandomPolicy",
    "ReinforcePolicy",
    "RolloutBuffer",
    "ScriptedPaddlePolicy",
    "discounted_returns",
    "frame_diff",
    "load_checkpoint",
    "normalize",
    "predict_intercept",
    "preprocess_frame",
    "reinforce_update",
    "save_checkpoint",
    "softmax",
]

__version__ = "0.1.0"
