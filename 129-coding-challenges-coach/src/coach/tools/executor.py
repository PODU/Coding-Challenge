"""Runs code snippets (Python, JavaScript or Go) for the ``run_code`` tool.

Each run gets a fresh temporary directory, a timeout and an output cap. This
is not real isolation; it only keeps runs from stepping on each other.
"""

from __future__ import annotations

import shutil
import subprocess
import sys
import tempfile
import time
from dataclasses import dataclass
from pathlib import Path

from ..llm import ToolSpec
from . import ToolRegistry

DEFAULT_TIMEOUT_S = 30.0
DEFAULT_OUTPUT_CAP = 64 * 1024  # bytes

LANGUAGES: dict[str, dict] = {
    # -E -s: ignore env vars and user site-packages, but keep the sandbox
    # directory on sys.path so test files can import the solution file.
    "python": {"file": "main.py", "argv": lambda p: [sys.executable, "-E", "-s", str(p)]},
    "javascript": {"file": "main.js", "argv": lambda p: ["node", str(p)]},
    "go": {"file": "main.go", "argv": lambda p: ["go", "run", str(p)]},
}


@dataclass(frozen=True)
class ExecutionResult:
    stdout: str
    stderr: str
    exit_code: int
    timed_out: bool = False
    truncated: bool = False
    duration_s: float = 0.0

    def as_dict(self) -> dict:
        return {
            "stdout": self.stdout,
            "stderr": self.stderr,
            "exit_code": self.exit_code,
            "timed_out": self.timed_out,
            "truncated": self.truncated,
            "duration_s": round(self.duration_s, 3),
        }


def language_available(language: str) -> bool:
    if language == "python":
        return True
    tool = {"javascript": "node", "go": "go"}.get(language)
    return tool is not None and shutil.which(tool) is not None


class LocalCodeExecutor:
    def __init__(
        self,
        *,
        timeout_s: float = DEFAULT_TIMEOUT_S,
        output_cap_bytes: int = DEFAULT_OUTPUT_CAP,
    ) -> None:
        self.timeout_s = timeout_s
        self.output_cap_bytes = output_cap_bytes

    def _cap(self, data: bytes) -> tuple[str, bool]:
        truncated = len(data) > self.output_cap_bytes
        if truncated:
            data = data[: self.output_cap_bytes]
        return data.decode("utf-8", errors="replace"), truncated

    def execute(
        self, code: str, language: str, *, extra_files: dict[str, str] | None = None
    ) -> ExecutionResult:
        lang = LANGUAGES.get(language)
        if lang is None:
            return ExecutionResult(
                stdout="",
                stderr=f"unsupported language: {language} "
                f"(supported: {', '.join(LANGUAGES)})",
                exit_code=2,
            )
        if not language_available(language):
            return ExecutionResult(
                stdout="",
                stderr=f"runtime for {language} is not installed on this host",
                exit_code=127,
            )
        with tempfile.TemporaryDirectory(prefix="coach-run-") as tmp:
            workdir = Path(tmp)
            main = workdir / lang["file"]
            main.write_text(code, encoding="utf-8")
            for name, content in (extra_files or {}).items():
                (workdir / name).write_text(content, encoding="utf-8")
            start = time.monotonic()
            try:
                proc = subprocess.run(
                    lang["argv"](main),
                    cwd=workdir,
                    capture_output=True,
                    timeout=self.timeout_s,
                )
            except subprocess.TimeoutExpired as exc:
                stdout, out_cut = self._cap(exc.stdout or b"")
                stderr, err_cut = self._cap(exc.stderr or b"")
                return ExecutionResult(
                    stdout=stdout,
                    stderr=stderr or f"execution timed out after {self.timeout_s}s",
                    exit_code=124,
                    timed_out=True,
                    truncated=out_cut or err_cut,
                    duration_s=time.monotonic() - start,
                )
            except FileNotFoundError:
                return ExecutionResult(
                    stdout="",
                    stderr=f"runtime for {language} is not installed on this host",
                    exit_code=127,
                )
            stdout, out_cut = self._cap(proc.stdout)
            stderr, err_cut = self._cap(proc.stderr)
            return ExecutionResult(
                stdout=stdout,
                stderr=stderr,
                exit_code=proc.returncode,
                truncated=out_cut or err_cut,
                duration_s=time.monotonic() - start,
            )


def register(registry: ToolRegistry, executor: LocalCodeExecutor) -> None:
    registry.register(
        ToolSpec(
            name="run_code",
            description=(
                "Run a code snippet in a fresh sandbox and return stdout, "
                "stderr and the exit code. Languages: python, javascript, go."
            ),
            parameters={
                "type": "object",
                "properties": {
                    "code": {"type": "string"},
                    "language": {
                        "type": "string",
                        "enum": list(LANGUAGES),
                    },
                },
                "required": ["code", "language"],
            },
        ),
        lambda code, language: executor.execute(code, language).as_dict(),
    )
