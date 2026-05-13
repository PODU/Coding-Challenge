"""Deterministic AI-vs-scripted simulation on the internal Pong engine."""

from __future__ import annotations

import numpy as np

from pong_ai.engine import PongEngine, predict_intercept
from pong_ai.policy import ScriptedPaddlePolicy


def test_predict_intercept_no_bounce():
    # Straight horizontal shot: intercept at the same height.
    y = predict_intercept(ball_x=0.0, ball_y=10.0, vx=1.0, vy=0.0,
                          paddle_x=40.0, field_height=20.0)
    assert abs(y - 10.0) < 1e-9


def test_predict_intercept_single_wall_reflection():
    # From y=15 rising at vy=1 over t=10 -> unfolded 25, reflects off the
    # top wall (H=20) back down to 15.
    y = predict_intercept(ball_x=0.0, ball_y=15.0, vx=1.0, vy=1.0,
                          paddle_x=10.0, field_height=20.0)
    assert abs(y - 15.0) < 1e-9


def test_predict_intercept_multi_bounce():
    # Unfolded landing 65 with H=20 -> fold(65) over period 40: 65%40=25,
    # 25>20 -> 40-25 = 15. (Several conceptual bounces.)
    y = predict_intercept(ball_x=0.0, ball_y=5.0, vx=1.0, vy=6.0,
                          paddle_x=10.0, field_height=20.0)
    assert abs(y - 15.0) < 1e-9


def test_predict_intercept_ball_moving_away_returns_fallback():
    y = predict_intercept(ball_x=20.0, ball_y=3.0, vx=-1.0, vy=1.0,
                          paddle_x=40.0, field_height=20.0, fallback_y=7.0)
    assert y == 7.0


def _rollout(steps: int, seed: int):
    """Scripted (capped) right paddle vs. a perfect (uncapped) left paddle.

    The left paddle tracks the ball's current height exactly and is tall enough
    (paddle_half > spin) that it never misses. The right paddle uses the capped
    trajectory-prediction policy, so a fast ball with seeded spin eventually
    out-runs its speed cap.
    """
    eng = PongEngine(width=40.0, height=20.0, paddle_half=2.5,
                     ball_speed_x=8.0, left_spin_vy_max=2.0, seed=seed)
    eng.reset(vy=1.5, toward_right=True)
    scripted = ScriptedPaddlePolicy(paddle_x=eng.width, field_height=eng.height,
                                    max_speed=1.0)
    trace = []
    for _ in range(steps):
        state = eng.state
        right_move = scripted.move(state)
        # Perfect left paddle: instantly tracks the ball (uncapped).
        left_move = state.ball_y - state.left_y
        new_state, scored = eng.step(left_move, right_move)
        trace.append((new_state.as_tuple(), round(right_move, 6), scored))
    return eng, trace


def test_rollout_is_bit_for_bit_reproducible():
    _, trace_a = _rollout(steps=200, seed=11)
    _, trace_b = _rollout(steps=200, seed=11)
    assert trace_a == trace_b


def test_scripted_paddle_intercepts_a_wall_reflected_ball():
    # Over the rollout the ball reflects off a wall and the capped paddle still
    # returns it at least once (vx flips from + to - on a right-paddle hit).
    eng, trace = _rollout(steps=200, seed=11)
    saw_wall_bounce = False
    saw_paddle_return = False
    prev_vy = None
    prev_vx = None
    for (state, _move, _scored) in trace:
        vx, vy = state[2], state[3]
        if prev_vy is not None and np.sign(vy) != np.sign(prev_vy):
            saw_wall_bounce = True
        if prev_vx is not None and prev_vx > 0 and vx < 0:
            saw_paddle_return = True  # right paddle sent it back
        prev_vy, prev_vx = vy, vx
    assert saw_wall_bounce, "expected at least one wall reflection"
    assert saw_paddle_return, "scripted paddle never returned the ball"


def test_capped_scripted_paddle_loses_to_perfect_mover():
    # The perfect (uncapped) left paddle never misses, so the capped scripted
    # right paddle concedes every point it cannot reach. `left_score` counts
    # points won by the left player (= times the scripted paddle missed).
    eng, _ = _rollout(steps=3000, seed=11)
    assert eng.right_score == 0, "the perfect mover should never be scored on"
    assert eng.left_score > eng.right_score, (
        f"scripted should lose: left={eng.left_score} right={eng.right_score}"
    )
