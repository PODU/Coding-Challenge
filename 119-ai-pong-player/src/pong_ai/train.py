"""Batched REINFORCE training on Atari Pong.

Needs the ``atari`` extra and the Atari ROM. Expect hours on a CPU.

Usage:
    pong-ai-train --episodes 10000 --batch-size 10 --lr 1e-3 --out runs/pong
"""

from __future__ import annotations

import argparse
import csv
import time
from pathlib import Path

import numpy as np

from .agent import RolloutBuffer, reinforce_update
from .checkpoint import load_checkpoint, save_checkpoint
from .network import Adam, PolicyNet
from .policy import PONG_ACTIONS, ReinforcePolicy
from .preprocess import frame_diff, preprocess_frame


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    p = argparse.ArgumentParser(description="Train a REINFORCE Pong agent.")
    p.add_argument("--episodes", type=int, default=10000)
    p.add_argument("--batch-size", type=int, default=10,
                   help="episodes per gradient update")
    p.add_argument("--lr", type=float, default=1e-3)
    p.add_argument("--gamma", type=float, default=0.99)
    p.add_argument("--hidden", type=int, default=200)
    p.add_argument("--seed", type=int, default=42)
    p.add_argument("--out", type=Path, default=Path("runs/pong"))
    p.add_argument("--checkpoint-every", type=int, default=50,
                   help="save a checkpoint every N episodes")
    p.add_argument("--resume", type=Path, default=None,
                   help="checkpoint to resume from")
    return p.parse_args(argv)


def run_episode(env, policy: ReinforcePolicy, buffer: RolloutBuffer) -> float:
    """Play one full episode, recording (obs, action, reward) into buffer."""
    obs, _ = env.reset()
    prev = None
    total = 0.0
    terminated = truncated = False
    while not (terminated or truncated):
        cur = preprocess_frame(obs)
        diff = frame_diff(cur, prev)
        prev = cur
        result = policy.act(diff, greedy=False)
        obs, reward, terminated, truncated, _ = env.step(PONG_ACTIONS[result.action])
        buffer.add(diff, result.action, float(reward))
        total += float(reward)
    return total


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    # Imported here so `--help` works without gymnasium installed.
    from .env import make_pong_env

    if args.resume is not None:
        net, _ = load_checkpoint(args.resume)
    else:
        net = PolicyNet(hidden_units=args.hidden,
                        n_actions=len(PONG_ACTIONS), seed=args.seed)
    optimizer = Adam(net, lr=args.lr)
    policy = ReinforcePolicy(net, seed=args.seed)

    env = make_pong_env(render_mode=None, seed=args.seed)
    args.out.mkdir(parents=True, exist_ok=True)
    log_path = args.out / "rewards.csv"
    with log_path.open("w", newline="") as fh:
        writer = csv.writer(fh)
        writer.writerow(["episode", "reward", "running_mean", "loss"])

        buffer = RolloutBuffer()
        running_mean: float | None = None
        for episode in range(1, args.episodes + 1):
            ep_reward = run_episode(env, policy, buffer)
            running_mean = (
                ep_reward if running_mean is None
                else 0.99 * running_mean + 0.01 * ep_reward
            )
            loss = float("nan")
            if episode % args.batch_size == 0:
                loss = reinforce_update(buffer, net, optimizer, gamma=args.gamma)
                buffer.clear()
            writer.writerow([episode, ep_reward, running_mean, loss])
            fh.flush()
            print(f"ep {episode:>6}  reward {ep_reward:+6.1f}  "
                  f"mean {running_mean:+6.2f}  loss {loss:.4f}")
            if episode % args.checkpoint_every == 0:
                save_checkpoint(
                    args.out / "checkpoint.npz", net,
                    {"episode": episode, "running_mean": running_mean,
                     "time": time.time()},
                )
    env.close()
    save_checkpoint(args.out / "final.npz", net, {"episode": args.episodes})
    print(f"done. rewards -> {log_path}")
    return 0


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
