# Coding Challenge #119 — AI Pong Player (REINFORCE from pixels)

A reinforcement-learning agent that learns to play **Atari Pong directly from
screen pixels** using the **REINFORCE** (vanilla policy-gradient) algorithm —
the classic Karpathy "Pong from Pixels" task on the modern
**Gymnasium / ALE-py** stack.

The package is split into a **pure functional core** (deterministic, headless,
NumPy-only) and an **imperative shell** (the Atari env, training loop, video /
plot I/O). The core is what the test-suite exercises; the shell is what you run
to actually train.

> **Why NumPy and not PyTorch?** The challenge brief targets PyTorch, but the
> test-suite is required to run **fully offline and fast** with no heavy deps
> and no Atari ROM download. The tiny one-hidden-layer policy net and its
> REINFORCE gradient are therefore implemented by hand in NumPy
> (`src/pong_ai/network.py`). The math is identical to what autograd would
> compute, so learning behaviour matches a PyTorch version. The Gymnasium
> environment is gated behind an optional import and is never touched by CI.

---

## Layout

```
src/pong_ai/
  preprocess.py   PURE: crop / grayscale / resize 80x80 / flatten / frame-diff
  returns.py      PURE: discounted returns (+ point-boundary reset) + normalize
  network.py      PURE: PolicyNet (Linear 6400->200->n_actions) fwd + REINFORCE backward + Adam
  policy.py       PURE: Policy interface, RandomPolicy, ReinforcePolicy, ScriptedPaddlePolicy
  agent.py        PURE: RolloutBuffer + reinforce_update
  engine.py       PURE: tiny deterministic Pong engine + trajectory prediction (test fixture)
  checkpoint.py   PURE: save / load model checkpoints (.npz)
  env.py          SHELL: make_pong_env (gymnasium / ale-py)  [optional dep]
  train.py        SHELL: batched REINFORCE training loop + checkpointing
  evaluate.py     SHELL: greedy eval, mp4 video, reward-curve plot
tests/            pure-core pytest suite (offline, deterministic)
```

The pipeline matches the challenge steps exactly:

1. **Random agent** — `RandomPolicy` (uniform over `{NOOP, UP, DOWN}`).
2. **Preprocessing** — `preprocess_frame`: crop the score/border, grayscale,
   downsample to `80x80`, flatten to `(6400,)` in `[0,1]`; `frame_diff` gives
   the current−previous difference (zeros at episode start).
3. **Policy net** — `PolicyNet`: `6400 → 200 (ReLU) → n_actions`, softmax;
   actions are **sampled** in training, **argmax** at eval.
4. **Returns** — `discounted_returns(gamma=0.99)` accumulates backwards and
   **resets the running sum on every scored point**; `normalize` standardizes.
5. **REINFORCE** — loss `mean(−log π(a|s) · advantage)`, batched over several
   episodes per update (`reinforce_update`).
6. **Visualization & checkpointing** — `.npz` checkpoints, greedy eval mode,
   `mp4` recording (imageio), reward-curve PNG (matplotlib).

---

## What is tested (offline, fast, deterministic)

The CI suite needs **only NumPy + pytest** — no GPU, no Atari ROM, no network:

```bash
pip install -e .[dev]      # or: pip install numpy pytest
python -m pytest           # ~1s, 30 tests
```

Coverage:

- **preprocess** — output shape `(6400,)`, dtype/range, a white ball pixel
  landing at the expected downsampled index, grayscale averaging, frame-diff.
- **returns** — hand-computed discounted vector **with point-boundary reset**;
  normalization to ~zero-mean/unit-std; zero-variance input yields no NaN.
- **policy** — `RandomPolicy` reproducibility; `PolicyNet.forward` shape;
  `ReinforcePolicy` greedy=argmax determinism, seeded-sampling reproducibility,
  and `log_prob == log softmax(logits)[action]`.
- **agent** — `reinforce_update` returns a finite loss and **strictly
  decreases** it across steps on a fixed batch (gradient flows / sign correct);
  positive advantage raises the chosen action's probability; checkpoint
  round-trip (forward pass identical after save/load).
- **sim** — a seeded internal Pong engine + `ScriptedPaddlePolicy`: trajectory
  prediction with multi-bounce wall reflection, bit-for-bit reproducible
  `(state, action)` trace, and the **capped** scripted paddle losing to a
  perfect mover (beatability / speed-cap check).

---

## How to actually train (the real Atari task — NOT run in CI)

Training needs the emulator stack and an Atari ROM, and takes **hours on CPU**.

```bash
# 1. Install the Atari extra (downloads ROMs under the ALE license).
pip install -e .[atari]

# 2. Train. Defaults: gamma=0.99, hidden=200, lr=1e-3, batch=10 episodes, seed=42.
pong-ai-train --episodes 10000 --batch-size 10 --lr 1e-3 --out runs/pong
#   -> runs/pong/rewards.csv, periodic runs/pong/checkpoint.npz, runs/pong/final.npz

# 3. Evaluate greedily, record a video, and plot the reward curve.
pong-ai-eval --checkpoint runs/pong/final.npz --episodes 5 \
    --video runs/pong/play.mp4 \
    --rewards runs/pong/rewards.csv --plot runs/pong/reward_curve.png
```

(`python -m pong_ai.train ...` / `python -m pong_ai.evaluate ...` work too.)

Average reward (point differential per game, range `−21 … +21`) should climb
from roughly `−21` toward and past `0` over many thousands of episodes. The
shell modules import fine **without** gymnasium; calling `make_pong_env`
without the `atari` extra raises a clear `ImportError` telling you what to
install.

---

## Default hyperparameters

`gamma=0.99`, hidden units `=200`, lr `=1e-3` (Adam), batch `=10` episodes per
update, action set `{NOOP, UP, DOWN}`, `seed=42`.
