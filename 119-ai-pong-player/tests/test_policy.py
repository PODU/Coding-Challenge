"""Tests for the Policy implementations + network forward pass."""

from __future__ import annotations

import numpy as np

from pong_ai.network import PolicyNet, softmax
from pong_ai.policy import (
    ActionResult,
    Policy,
    RandomPolicy,
    ReinforcePolicy,
)
from pong_ai.preprocess import OBS_DIM


def test_random_policy_is_reproducible_and_in_range():
    p1 = RandomPolicy(n_actions=3, seed=7)
    p2 = RandomPolicy(n_actions=3, seed=7)
    obs = np.zeros(OBS_DIM, dtype=np.float32)
    seq1 = [p1.act(obs).action for _ in range(20)]
    seq2 = [p2.act(obs).action for _ in range(20)]
    assert seq1 == seq2
    assert all(0 <= a < 3 for a in seq1)
    assert len(set(seq1)) > 1  # actually varies


def test_random_policy_satisfies_protocol():
    assert isinstance(RandomPolicy(), Policy)
    assert isinstance(ReinforcePolicy(PolicyNet(seed=0)), Policy)


def test_network_forward_shape():
    net = PolicyNet(n_actions=3, seed=1)
    assert net.num_parameters == 6400 * 200 + 200 + 200 * 3 + 3
    batch = np.zeros((5, OBS_DIM), dtype=np.float32)
    logits = net.forward(batch)
    assert logits.shape == (5, 3)
    # single-observation forward broadcasts to (1, A)
    single = net.forward(np.zeros(OBS_DIM, dtype=np.float32))
    assert single.shape == (1, 3)


def test_reinforce_greedy_is_deterministic_argmax():
    net = PolicyNet(n_actions=3, seed=3)
    obs = np.random.default_rng(0).standard_normal(OBS_DIM).astype(np.float32)
    policy = ReinforcePolicy(net, seed=99)
    expected = int(np.argmax(softmax(net.forward(obs))[0]))
    for _ in range(5):
        assert policy.act(obs, greedy=True).action == expected


def test_reinforce_sampling_is_reproducible():
    net = PolicyNet(n_actions=3, seed=3)
    obs = np.random.default_rng(1).standard_normal(OBS_DIM).astype(np.float32)
    a = ReinforcePolicy(net, seed=123)
    b = ReinforcePolicy(net, seed=123)
    seq_a = [a.act(obs, greedy=False).action for _ in range(30)]
    seq_b = [b.act(obs, greedy=False).action for _ in range(30)]
    assert seq_a == seq_b


def test_reinforce_log_prob_matches_log_softmax():
    net = PolicyNet(n_actions=3, seed=5)
    obs = np.random.default_rng(2).standard_normal(OBS_DIM).astype(np.float32)
    policy = ReinforcePolicy(net, seed=11)
    res = policy.act(obs, greedy=False)
    assert isinstance(res, ActionResult)
    probs = softmax(net.forward(obs))[0]
    expected = float(np.log(probs[res.action]))
    assert abs(res.log_prob - expected) < 1e-5
