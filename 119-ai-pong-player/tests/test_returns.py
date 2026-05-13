"""Tests for discounted returns + normalization."""

from __future__ import annotations

import numpy as np

from pong_ai.returns import discounted_returns, normalize


def test_hand_computed_with_point_boundary_reset():
    rewards = [0, 0, 1, 0, -1]
    out = discounted_returns(rewards, gamma=0.9)
    # Backward, resetting the accumulator on each non-zero reward:
    #   t=4: reset -> -1
    #   t=3: -0.9*1 + 0 = -0.9
    #   t=2: reset -> +1
    #   t=1: 0.9*1 = 0.9
    #   t=0: 0.9*0.9 = 0.81
    expected = np.array([0.81, 0.9, 1.0, -0.9, -1.0], dtype=np.float32)
    np.testing.assert_allclose(out, expected, atol=1e-6)


def test_single_reward_at_end():
    out = discounted_returns([0, 0, 0, 1], gamma=0.99)
    expected = np.array([0.99**3, 0.99**2, 0.99, 1.0], dtype=np.float32)
    np.testing.assert_allclose(out, expected, atol=1e-6)


def test_two_independent_points_do_not_bleed():
    # The +1 must NOT influence the second segment's accumulation.
    out = discounted_returns([0, 1, 0, 0], gamma=0.5)
    expected = np.array([0.5, 1.0, 0.0, 0.0], dtype=np.float32)
    np.testing.assert_allclose(out, expected, atol=1e-6)


def test_normalize_zero_mean_unit_std():
    out = normalize(np.array([1.0, 2.0, 3.0, 4.0, 5.0]))
    assert abs(float(out.mean())) < 1e-5
    assert abs(float(out.std()) - 1.0) < 1e-3


def test_normalize_zero_variance_no_nan():
    out = normalize(np.array([5.0, 5.0, 5.0]))
    assert np.all(np.isfinite(out))
    np.testing.assert_allclose(out, 0.0, atol=1e-5)
