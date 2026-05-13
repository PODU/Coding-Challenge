"""Discounted returns for Pong.

Every non-zero reward ends a point, and each point is credited separately, so
the running sum is reset at a non-zero reward before that reward is added. Get
the order wrong and credit leaks from one point into the previous one.
"""

from __future__ import annotations

from collections.abc import Sequence

import numpy as np


def discounted_returns(
    rewards: Sequence[float] | np.ndarray, gamma: float = 0.99
) -> np.ndarray:
    """Discounted returns, with the running sum reset at every scored point."""
    r = np.asarray(rewards, dtype=np.float32).ravel()
    out = np.zeros_like(r)
    running = 0.0
    for t in range(len(r) - 1, -1, -1):
        if r[t] != 0.0:
            running = 0.0
        running = running * gamma + float(r[t])
        out[t] = running
    return out


def normalize(returns: np.ndarray, eps: float = 1e-8) -> np.ndarray:
    """Standardize to zero mean and unit std. ``eps`` avoids NaNs when all
    returns are equal."""
    r = np.asarray(returns, dtype=np.float32).ravel()
    return ((r - r.mean()) / (r.std() + eps)).astype(np.float32)
