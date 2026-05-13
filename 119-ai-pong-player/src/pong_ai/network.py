"""One-hidden-layer policy network with a hand-written REINFORCE backward pass.

    Linear(6400 -> 200) -> ReLU -> Linear(200 -> n_actions) -> softmax

Written in NumPy rather than PyTorch so the tests only need NumPy. The gradient
is the same one autograd would produce.
"""

from __future__ import annotations

from dataclasses import dataclass

import numpy as np

INPUT_DIM = 6400
HIDDEN_UNITS = 200


def softmax(logits: np.ndarray) -> np.ndarray:
    """Numerically stable softmax over the last axis."""
    logits = np.asarray(logits, dtype=np.float32)
    z = logits - logits.max(axis=-1, keepdims=True)
    e = np.exp(z)
    return (e / e.sum(axis=-1, keepdims=True)).astype(np.float32)


@dataclass
class _ForwardCache:

    x: np.ndarray      # (N, input_dim)
    z1: np.ndarray     # (N, hidden) pre-activation
    h: np.ndarray      # (N, hidden) post-ReLU
    logits: np.ndarray  # (N, n_actions)


class PolicyNet:
    """MLP mapping a 6400-vector to action logits. Weights are seeded."""

    def __init__(
        self,
        input_dim: int = INPUT_DIM,
        hidden_units: int = HIDDEN_UNITS,
        n_actions: int = 3,
        seed: int = 42,
    ) -> None:
        self.input_dim = input_dim
        self.hidden_units = hidden_units
        self.n_actions = n_actions
        rng = np.random.default_rng(seed)
        self.W1 = (
            rng.standard_normal((hidden_units, input_dim)) / np.sqrt(input_dim)
        ).astype(np.float32)
        self.b1 = np.zeros(hidden_units, dtype=np.float32)
        self.W2 = (
            rng.standard_normal((n_actions, hidden_units)) / np.sqrt(hidden_units)
        ).astype(np.float32)
        self.b2 = np.zeros(n_actions, dtype=np.float32)

    def forward(
        self, x: np.ndarray, *, cache: bool = False
    ) -> np.ndarray | tuple[np.ndarray, _ForwardCache]:
        """Return ``(N, n_actions)`` logits, plus the activations if ``cache``."""
        arr = np.asarray(x, dtype=np.float32)
        if arr.ndim == 1:
            arr = arr[None, :]
        z1 = arr @ self.W1.T + self.b1
        h = np.maximum(z1, 0.0)
        logits = h @ self.W2.T + self.b2
        if cache:
            return logits, _ForwardCache(x=arr, z1=z1, h=h, logits=logits)
        return logits

    def probabilities(self, x: np.ndarray) -> np.ndarray:
        return softmax(self.forward(x))

    def reinforce_gradients(
        self,
        cache: _ForwardCache,
        actions: np.ndarray,
        advantages: np.ndarray,
    ) -> tuple[dict[str, np.ndarray], float]:
        """Return ``(grads, loss)`` for the loss ``mean(-log pi(a|s) * advantage)``."""
        actions = np.asarray(actions, dtype=np.int64).ravel()
        advantages = np.asarray(advantages, dtype=np.float32).ravel()
        n = cache.x.shape[0]
        if not (len(actions) == len(advantages) == n):
            raise ValueError("actions, advantages and batch size must match")

        probs = softmax(cache.logits)
        idx = np.arange(n)
        chosen = np.clip(probs[idx, actions], 1e-12, 1.0)
        loss = float(np.mean(-np.log(chosen) * advantages))

        # d loss / d logits = (1/N) * advantage * (softmax - onehot)
        dlogits = probs.copy()
        dlogits[idx, actions] -= 1.0
        dlogits *= advantages[:, None]
        dlogits /= n

        dW2 = dlogits.T @ cache.h
        db2 = dlogits.sum(axis=0)

        dh = dlogits @ self.W2
        dz1 = dh * (cache.z1 > 0.0)
        dW1 = dz1.T @ cache.x
        db1 = dz1.sum(axis=0)

        grads = {
            "W1": dW1.astype(np.float32),
            "b1": db1.astype(np.float32),
            "W2": dW2.astype(np.float32),
            "b2": db2.astype(np.float32),
        }
        return grads, loss

    def state_dict(self) -> dict[str, np.ndarray]:
        return {
            "W1": self.W1.copy(),
            "b1": self.b1.copy(),
            "W2": self.W2.copy(),
            "b2": self.b2.copy(),
        }

    def load_state_dict(self, state: dict[str, np.ndarray]) -> None:
        self.W1 = np.asarray(state["W1"], dtype=np.float32)
        self.b1 = np.asarray(state["b1"], dtype=np.float32)
        self.W2 = np.asarray(state["W2"], dtype=np.float32)
        self.b2 = np.asarray(state["b2"], dtype=np.float32)
        self.hidden_units, self.input_dim = self.W1.shape
        self.n_actions = self.W2.shape[0]

    @property
    def num_parameters(self) -> int:
        return self.W1.size + self.b1.size + self.W2.size + self.b2.size


class Adam:
    """Minimal Adam optimizer over a PolicyNet's named parameters."""

    def __init__(
        self,
        net: PolicyNet,
        lr: float = 1e-3,
        betas: tuple[float, float] = (0.9, 0.999),
        eps: float = 1e-8,
    ) -> None:
        self.net = net
        self.lr = lr
        self.b1, self.b2 = betas
        self.eps = eps
        self.t = 0
        self._m = {k: np.zeros_like(v) for k, v in net.state_dict().items()}
        self._v = {k: np.zeros_like(v) for k, v in net.state_dict().items()}

    def step(self, grads: dict[str, np.ndarray]) -> None:
        self.t += 1
        for name, grad in grads.items():
            param = getattr(self.net, name)
            m = self._m[name] = self.b1 * self._m[name] + (1 - self.b1) * grad
            v = self._v[name] = self.b2 * self._v[name] + (1 - self.b2) * (grad * grad)
            m_hat = m / (1 - self.b1**self.t)
            v_hat = v / (1 - self.b2**self.t)
            param -= self.lr * m_hat / (np.sqrt(v_hat) + self.eps)
