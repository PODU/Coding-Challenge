"""Agent backends. Each has one method, invoke(prompt) -> text.

MockBackend: canned responses held in memory, for unit tests.
DirMockBackend: canned responses read from files in a directory. Its position
    is saved in the run dir so a resumed run picks up where it left off.
ClaudeCLIBackend: runs `claude -p ... --output-format text`.
"""

from __future__ import annotations

import subprocess
from pathlib import Path
from typing import Protocol

from .config import RunConfig


class BackendError(RuntimeError):
    pass


class AgentBackend(Protocol):
    def invoke(self, prompt: str) -> str: ...


class MockBackend:
    def __init__(self, responses: list[str]):
        self.responses = list(responses)
        self.calls: list[str] = []

    def invoke(self, prompt: str) -> str:
        self.calls.append(prompt)
        if not self.responses:
            raise BackendError("mock backend exhausted")
        return self.responses.pop(0)


class DirMockBackend:
    """Returns the files in mock_dir one at a time, in name order."""

    CURSOR = "mock_cursor"

    def __init__(self, mock_dir: Path, run_dir: Path):
        self.files = sorted(p for p in Path(mock_dir).iterdir() if p.is_file())
        if not self.files:
            raise BackendError(f"no mock responses in {mock_dir}")
        self.cursor_file = Path(run_dir) / self.CURSOR

    def invoke(self, prompt: str) -> str:
        idx = 0
        if self.cursor_file.is_file():
            idx = int(self.cursor_file.read_text().strip() or 0)
        if idx >= len(self.files):
            raise BackendError("mock responses exhausted")
        text = self.files[idx].read_text(encoding="utf-8")
        self.cursor_file.write_text(str(idx + 1))
        return text


class ClaudeCLIBackend:
    def __init__(self, cmd: str = "claude", extra_args: list[str] | None = None,
                 timeout: int = 600):
        self.cmd = cmd
        self.extra_args = extra_args or []
        self.timeout = timeout

    def invoke(self, prompt: str) -> str:
        argv = [self.cmd, "-p", prompt, "--output-format", "text",
                *self.extra_args]
        try:
            proc = subprocess.run(argv, capture_output=True, text=True,
                                  timeout=self.timeout)
        except FileNotFoundError as e:
            raise BackendError(f"agent CLI not found: {self.cmd}") from e
        except subprocess.TimeoutExpired as e:
            raise BackendError(f"agent CLI timed out after {self.timeout}s") from e
        if proc.returncode != 0:
            raise BackendError(
                f"agent CLI exit {proc.returncode}: {proc.stderr.strip()[:500]}")
        return proc.stdout


def make_backend(cfg: RunConfig, run_dir: Path) -> AgentBackend:
    if cfg.backend == "mock":
        if not cfg.mock_dir:
            raise BackendError("backend=mock requires mock_dir")
        mock_dir = Path(cfg.mock_dir)
        # Fan-out candidates can have their own script in mock_dir/<cand-name>.
        cand_sub = mock_dir / run_dir.name
        if cand_sub.is_dir():
            mock_dir = cand_sub
        return DirMockBackend(mock_dir, run_dir)
    if cfg.backend == "claude":
        return ClaudeCLIBackend(cfg.claude_cmd, cfg.claude_args, cfg.agent_timeout)
    raise BackendError(f"unknown backend: {cfg.backend}")
