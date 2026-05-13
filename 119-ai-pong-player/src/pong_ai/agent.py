"""REINFORCE rollout buffer and update step."""

from __future__ import annotations

from dataclasses import dataclass, field

import numpy as np

from .network import Adam, PolicyNet
from .returns import discounted_returns, normalize


@dataclass
class RolloutBuffer:
    """Accumulates ``(observation, action, reward)`` over an episode/batch."""

    observations: list[np.ndarray] = field(default_factory=list)
    actions: list[int] = field(default_factory=list)
    rewards: list[float] = field(default_factory=list)

    def add(self, observation: np.ndarray, action: int, reward: float) -> None:
        self.observations.append(np.asarray(observation, dtype=np.float32))
        self.actions.append(int(action))
        self.rewards.append(float(reward))

    def clear(self) -> None:
        self.observations.clear()
        self.actions.clear()
        self.rewards.clear()

    def __len__(self) -> int:
        return len(self.actions)

    def as_arrays(self) -> tuple[np.ndarray, np.ndarray, np.ndarray]:
        return (
            np.asarray(self.observations, dtype=np.float32),
            np.asarray(self.actions, dtype=np.int64),
            np.asarray(self.rewards, dtype=np.float32),
        )


def reinforce_update(
    buffer: RolloutBuffer,
    net: PolicyNet,
    optimizer: Adam,
    gamma: float = 0.99,
) -> float:
    """Run one REINFORCE update from a filled buffer and return the loss.

    The loss is ``mean(-log pi(a|s) * advantage)`` with normalized discounted
    returns as advantages. If the loss or any gradient is non-finite the
    optimizer step is skipped so one bad batch cannot poison the weights.
    """
    if len(buffer) == 0:
        raise ValueError("cannot update from an empty buffer")

    obs, actions, rewards = buffer.as_arrays()
    returns = discounted_returns(rewards, gamma=gamma)
    advantages = normalize(returns)

    logits, cache = net.forward(obs, cache=True)
    grads, loss = net.reinforce_gradients(cache, actions, advantages)

    if np.isfinite(loss) and all(np.all(np.isfinite(g)) for g in grads.values()):
        optimizer.step(grads)
    return float(loss)
