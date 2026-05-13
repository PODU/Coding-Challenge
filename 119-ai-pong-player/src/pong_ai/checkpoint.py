"""PolicyNet checkpoints: ``.npz`` archives of the weights plus JSON metadata."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

import numpy as np

from .network import PolicyNet


def save_checkpoint(
    path: str | Path,
    net: PolicyNet,
    meta: dict[str, Any] | None = None,
) -> Path:
    """Write ``net``'s parameters (+ optional metadata) to ``path`` (.npz)."""
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    meta = dict(meta or {})
    meta.setdefault("input_dim", net.input_dim)
    meta.setdefault("hidden_units", net.hidden_units)
    meta.setdefault("n_actions", net.n_actions)
    arrays = net.state_dict()
    np.savez(path, _meta=np.frombuffer(
        json.dumps(meta).encode("utf-8"), dtype=np.uint8), **arrays)
    # np.savez appends .npz if missing; normalize the returned path.
    if path.suffix != ".npz":
        path = path.with_suffix(path.suffix + ".npz")
    return path


def load_checkpoint(path: str | Path) -> tuple[PolicyNet, dict[str, Any]]:
    """Load a checkpoint into a fresh PolicyNet. Returns ``(net, meta)``."""
    path = Path(path)
    if path.suffix != ".npz" and not path.exists():
        path = path.with_suffix(path.suffix + ".npz")
    data = np.load(path, allow_pickle=False)
    meta = json.loads(bytes(data["_meta"].tobytes()).decode("utf-8"))
    net = PolicyNet(
        input_dim=int(meta["input_dim"]),
        hidden_units=int(meta["hidden_units"]),
        n_actions=int(meta["n_actions"]),
    )
    net.load_state_dict({k: data[k] for k in ("W1", "b1", "W2", "b2")})
    return net, meta
