import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "src"))

from agentloop.backend import MockBackend  # noqa: E402
from agentloop.config import RunConfig  # noqa: E402
from agentloop.state import RunState  # noqa: E402
from agentloop.workspace import copy_project  # noqa: E402

FIXTURE = Path(__file__).resolve().parents[1] / "fixture" / "buggy_calc"
MOCK_SESSION = Path(__file__).resolve().parents[1] / "fixture" / "mock_session"

PLAN = "1. fix subtract\n2. fix divide\n"

# Fixes subtract, leaves the divide bug — tests still fail once.
CALC_PARTIAL = """\
=== FILE: calc.py ===
def add(a, b):
    return a + b


def subtract(a, b):
    return a - b


def multiply(a, b):
    return a * b


def divide(a, b):
    return a / b
=== END FILE ===
"""

# Both bugs fixed — tests pass.
CALC_FULL = """\
=== FILE: calc.py ===
def add(a, b):
    return a + b


def subtract(a, b):
    return a - b


def multiply(a, b):
    return a * b


def divide(a, b):
    if b == 0:
        return None
    return a / b
=== END FILE ===
"""


def make_cfg(**kwargs) -> RunConfig:
    defaults = dict(task="fix the calculator bugs", project=str(FIXTURE),
                    backend="mock", auto_approve=True)
    defaults.update(kwargs)
    return RunConfig(**defaults)


def init_run(tmp_path: Path, cfg: RunConfig, run_id: str = "r1") -> Path:
    run_dir = tmp_path / "runs" / run_id
    run_dir.mkdir(parents=True)
    copy_project(Path(cfg.project), run_dir / "workspace")
    RunState.new(cfg, run_id=run_id).save(run_dir)
    return run_dir


def backend_map_factory(mapping: dict[str, MockBackend]):
    """Factory returning one persistent MockBackend per run/candidate name."""
    def factory(cfg, run_dir):
        return mapping[Path(run_dir).name]
    return factory


@pytest.fixture
def cfg():
    return make_cfg()
