"""Greedy evaluation, video recording and reward-curve plotting.

Needs the ``atari`` extra (and matplotlib for the plot).

Usage:
    pong-ai-eval --checkpoint runs/pong/final.npz --episodes 5 \
        --video runs/pong/play.mp4 --rewards runs/pong/rewards.csv \
        --plot runs/pong/reward_curve.png
"""

from __future__ import annotations

import argparse
import csv
from pathlib import Path

import numpy as np

from .checkpoint import load_checkpoint
from .policy import PONG_ACTIONS, ReinforcePolicy
from .preprocess import frame_diff, preprocess_frame


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    p = argparse.ArgumentParser(description="Evaluate / visualize a Pong agent.")
    p.add_argument("--checkpoint", type=Path, required=False)
    p.add_argument("--episodes", type=int, default=5)
    p.add_argument("--seed", type=int, default=42)
    p.add_argument("--video", type=Path, default=None,
                   help="write an mp4 of one greedy episode")
    p.add_argument("--rewards", type=Path, default=None,
                   help="rewards.csv produced by training (for the plot)")
    p.add_argument("--plot", type=Path, default=None,
                   help="write a reward-curve PNG from --rewards")
    return p.parse_args(argv)


def evaluate(checkpoint: Path, episodes: int, seed: int,
             video: Path | None) -> list[float]:
    """Run greedy episodes; optionally record the first one to ``video``."""
    from .env import make_pong_env

    net, _ = load_checkpoint(checkpoint)
    policy = ReinforcePolicy(net, seed=seed)
    render_mode = "rgb_array" if video is not None else None
    env = make_pong_env(render_mode=render_mode, seed=seed)

    frames: list[np.ndarray] = []
    rewards: list[float] = []
    for ep in range(episodes):
        obs, _ = env.reset()
        prev = None
        total = 0.0
        terminated = truncated = False
        while not (terminated or truncated):
            if video is not None and ep == 0:
                frames.append(env.render())
            cur = preprocess_frame(obs)
            diff = frame_diff(cur, prev)
            prev = cur
            action = policy.act(diff, greedy=True).action
            obs, reward, terminated, truncated, _ = env.step(PONG_ACTIONS[action])
            total += float(reward)
        rewards.append(total)
    env.close()

    if video is not None and frames:
        _write_video(video, frames)
    return rewards


def _write_video(path: Path, frames: list[np.ndarray]) -> None:
    import imageio.v2 as imageio

    path.parent.mkdir(parents=True, exist_ok=True)
    imageio.mimsave(path, frames, fps=30)


def plot_rewards(rewards_csv: Path, out_png: Path) -> None:
    """Render the training reward curve from a rewards.csv log."""
    import matplotlib
    matplotlib.use("Agg")
    import matplotlib.pyplot as plt

    episodes: list[int] = []
    rewards: list[float] = []
    means: list[float] = []
    with rewards_csv.open(newline="") as fh:
        for row in csv.DictReader(fh):
            episodes.append(int(row["episode"]))
            rewards.append(float(row["reward"]))
            means.append(float(row["running_mean"]))

    out_png.parent.mkdir(parents=True, exist_ok=True)
    fig, ax = plt.subplots(figsize=(9, 5))
    ax.plot(episodes, rewards, alpha=0.3, label="episode reward")
    ax.plot(episodes, means, color="C1", label="running mean")
    ax.axhline(0.0, color="grey", lw=0.8, ls="--")
    ax.set_xlabel("episode")
    ax.set_ylabel("reward (point differential)")
    ax.set_title("REINFORCE Pong training reward")
    ax.legend()
    fig.tight_layout()
    fig.savefig(out_png, dpi=120)
    plt.close(fig)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    if args.checkpoint is not None:
        rewards = evaluate(args.checkpoint, args.episodes, args.seed, args.video)
        print(f"greedy rewards: {rewards}  mean {np.mean(rewards):+.2f}")
    if args.rewards is not None and args.plot is not None:
        plot_rewards(args.rewards, args.plot)
        print(f"reward curve -> {args.plot}")
    return 0


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
