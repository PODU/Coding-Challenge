"""Run state, saved to disk after every step so a run can be resumed.

A run directory holds state.json (status and config), plan.md,
last_failure.txt and transitions.jsonl (the timeline).
"""

from __future__ import annotations

import json
import os
import secrets
import time
from dataclasses import asdict, dataclass, field
from pathlib import Path

from .config import RunConfig

PLANNING = "planning"
AWAITING_APPROVAL = "awaiting_approval"
CODING = "coding"
TESTING = "testing"
FIXING = "fixing"
REVIEWING = "reviewing"
FANOUT = "fanout"
JOINING = "joining"
DONE = "done"
FAILED = "failed"

TERMINAL = {DONE, FAILED}

STATE_FILE = "state.json"
TRANSITIONS_FILE = "transitions.jsonl"
PLAN_FILE = "plan.md"
FAILURE_FILE = "last_failure.txt"


def new_run_id() -> str:
    return time.strftime("%Y%m%d-%H%M%S") + "-" + secrets.token_hex(2)


def now_iso() -> str:
    return time.strftime("%Y-%m-%dT%H:%M:%S")


@dataclass
class RunState:
    run_id: str
    task: str
    status: str
    config: dict = field(default_factory=dict)
    approved: bool = False
    iteration: int = 0
    tests_passed: bool | None = None
    gate_passed: bool | None = None
    winner: str | None = None
    error: str | None = None
    created: str = field(default_factory=now_iso)
    updated: str = field(default_factory=now_iso)

    @classmethod
    def new(cls, cfg: RunConfig, run_id: str | None = None,
            status: str = PLANNING) -> "RunState":
        return cls(run_id=run_id or new_run_id(), task=cfg.task,
                   status=status, config=cfg.to_dict())

    @property
    def cfg(self) -> RunConfig:
        return RunConfig.from_dict(self.config)

    def save(self, run_dir: Path) -> None:
        self.updated = now_iso()
        # Write then rename, so a crash never leaves a half-written state.json.
        tmp = run_dir / (STATE_FILE + ".tmp")
        tmp.write_text(json.dumps(asdict(self), indent=2), encoding="utf-8")
        os.replace(tmp, run_dir / STATE_FILE)

    @classmethod
    def load(cls, run_dir: Path) -> "RunState":
        d = json.loads((run_dir / STATE_FILE).read_text(encoding="utf-8"))
        known = {f for f in cls.__dataclass_fields__}
        return cls(**{k: v for k, v in d.items() if k in known})

    @classmethod
    def exists(cls, run_dir: Path) -> bool:
        return (run_dir / STATE_FILE).is_file()


def append_transition(run_dir: Path, node: str, frm: str, to: str,
                      duration: float, detail: str = "") -> None:
    entry = {"ts": now_iso(), "node": node, "from": frm, "to": to,
             "duration_s": round(duration, 3), "detail": detail}
    with open(run_dir / TRANSITIONS_FILE, "a", encoding="utf-8") as f:
        f.write(json.dumps(entry) + "\n")


def read_transitions(run_dir: Path) -> list[dict]:
    path = run_dir / TRANSITIONS_FILE
    if not path.is_file():
        return []
    out = []
    for line in path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if line:
            out.append(json.loads(line))
    return out


def list_runs(runs_dir: Path) -> list[RunState]:
    """Manifest of past runs, newest first."""
    runs = []
    if not runs_dir.is_dir():
        return runs
    for child in sorted(runs_dir.iterdir(), reverse=True):
        if child.is_dir() and RunState.exists(child):
            try:
                runs.append(RunState.load(child))
            except (json.JSONDecodeError, TypeError):
                continue
    return runs
