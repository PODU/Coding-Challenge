"""Gymnasium/ALE Pong environment. Needs the ``atari`` extra; tests never import it."""

from __future__ import annotations

from typing import Any

_IMPORT_HINT = (
    "Atari training requires the optional dependencies. Install them with:\n"
    "    pip install -e .[atari]\n"
    "and accept the Atari ROM license (gymnasium[atari] / ale-py)."
)


def make_pong_env(render_mode: str | None = None, seed: int | None = 42) -> Any:
    """Create ``ALE/Pong-v5``. Use ``render_mode="rgb_array"`` to record video.

    Raises ImportError with install instructions if gymnasium/ale-py are missing.
    """
    try:
        import ale_py
        import gymnasium as gym
    except ImportError as exc:  # pragma: no cover - exercised only with deps
        raise ImportError(_IMPORT_HINT) from exc

    gym.register_envs(ale_py)
    env = gym.make("ALE/Pong-v5", render_mode=render_mode)
    if seed is not None:
        env.reset(seed=seed)
        env.action_space.seed(seed)
        env.observation_space.seed(seed)
    return env
