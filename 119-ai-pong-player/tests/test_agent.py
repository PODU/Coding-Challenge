"""Tests for the REINFORCE update + checkpoint round-trip."""

from __future__ import annotations

import numpy as np

from pong_ai.agent import RolloutBuffer, reinforce_update
from pong_ai.checkpoint import load_checkpoint, save_checkpoint
from pong_ai.network import Adam, PolicyNet
from pong_ai.preprocess import OBS_DIM


def _tiny_buffer(seed: int = 0, input_dim: int = OBS_DIM) -> RolloutBuffer:
    rng = np.random.default_rng(seed)
    buf = RolloutBuffer()
    # 6 steps spanning two "points" (non-zero rewards at the boundaries).
    rewards = [0.0, 0.0, 1.0, 0.0, 0.0, -1.0]
    for t, r in enumerate(rewards):
        obs = (rng.standard_normal(input_dim) * 0.5).astype(np.float32)
        buf.add(obs, action=t % 3, reward=r)
    return buf


def test_reinforce_update_returns_finite_loss():
    net = PolicyNet(n_actions=3, seed=1)
    opt = Adam(net, lr=1e-3)
    loss = reinforce_update(_tiny_buffer(), net, opt, gamma=0.99)
    assert np.isfinite(loss)


def test_reinforce_update_strictly_decreases_loss():
    # Same fixed batch repeatedly -> gradient descent must reduce the loss.
    # Small, well-conditioned net (low input fan-in) so the step is gradual.
    net = PolicyNet(input_dim=16, hidden_units=16, n_actions=3, seed=2)
    opt = Adam(net, lr=1e-3)
    buf = _tiny_buffer(seed=42, input_dim=16)
    losses = [reinforce_update(buf, net, opt, gamma=0.99) for _ in range(8)]
    for earlier, later in zip(losses, losses[1:]):
        assert later < earlier, f"loss did not decrease: {losses}"


def test_gradient_sign_pushes_probability_toward_positive_advantage():
    # A single sample with positive advantage must increase that action's prob.
    net = PolicyNet(n_actions=3, seed=4)
    obs = np.random.default_rng(0).standard_normal((1, OBS_DIM)).astype(np.float32)
    action = np.array([1])
    advantage = np.array([1.0], dtype=np.float32)

    _, cache = net.forward(obs, cache=True)
    p_before = net.probabilities(obs)[0, 1]
    grads, _ = net.reinforce_gradients(cache, action, advantage)
    Adam(net, lr=1e-2).step(grads)
    p_after = net.probabilities(obs)[0, 1]
    assert p_after > p_before


def test_empty_buffer_raises():
    net = PolicyNet(seed=0)
    try:
        reinforce_update(RolloutBuffer(), net, Adam(net))
    except ValueError:
        pass
    else:  # pragma: no cover
        raise AssertionError("expected ValueError on empty buffer")


def test_checkpoint_roundtrip(tmp_path):
    net = PolicyNet(n_actions=3, seed=7)
    # perturb so weights differ from a fresh init
    net.W1 += 0.01
    path = save_checkpoint(tmp_path / "ckpt", net, {"episode": 5})
    assert path.exists()

    loaded, meta = load_checkpoint(path)
    assert meta["episode"] == 5
    assert meta["n_actions"] == 3
    for name in ("W1", "b1", "W2", "b2"):
        np.testing.assert_array_equal(
            getattr(net, name), getattr(loaded, name)
        )
    # forward pass identical
    obs = np.random.default_rng(0).standard_normal(OBS_DIM).astype(np.float32)
    np.testing.assert_allclose(net.forward(obs), loaded.forward(obs), atol=1e-7)


def test_checkpoint_roundtrip_without_extension(tmp_path):
    net = PolicyNet(seed=1)
    save_checkpoint(tmp_path / "model", net)
    loaded, _ = load_checkpoint(tmp_path / "model")
    np.testing.assert_array_equal(net.W2, loaded.W2)
