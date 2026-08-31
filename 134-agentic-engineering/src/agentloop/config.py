"""Run configuration, persisted inside state.json so a resume can rebuild everything."""

from __future__ import annotations

import shlex
import sys
from dataclasses import asdict, dataclass, field


def default_test_cmd() -> list[str]:
    return [sys.executable, "-m", "pytest", "-q"]


@dataclass
class RunConfig:
    task: str
    project: str
    backend: str = "mock"  # mock | claude
    mock_dir: str | None = None
    claude_cmd: str = "claude"
    claude_args: list[str] = field(default_factory=list)
    test_cmd: list[str] = field(default_factory=default_test_cmd)
    gate: str = "none"  # none | mock | sonar
    gate_fail_times: int = 0
    sonar_host: str = "http://localhost:9000"
    sonar_token: str = ""
    sonar_project_key: str = ""
    sonar_scanner: str = "sonar-scanner"
    max_iterations: int = 3
    candidates: int = 1
    auto_approve: bool = False
    agent_timeout: int = 600

    def to_dict(self) -> dict:
        return asdict(self)

    @classmethod
    def from_dict(cls, d: dict) -> "RunConfig":
        known = {f for f in cls.__dataclass_fields__}
        return cls(**{k: v for k, v in d.items() if k in known})

    @staticmethod
    def parse_cmd(cmd: str) -> list[str]:
        return shlex.split(cmd)
