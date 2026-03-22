"""run_command tool: run a command in the workspace root.

Every command goes through the :class:`PermissionPolicy` first. The command is
split with shlex and run without a shell, under a timeout, and long output is
truncated.
"""

from __future__ import annotations

import shlex
import subprocess
from typing import Any

from agent.errors import ToolError
from agent.permissions import PermissionPolicy
from agent.workspace import Workspace

DEFAULT_TIMEOUT_SECONDS = 30
MAX_OUTPUT_CHARS = 20_000


class RunCommandTool:
    name = "run_command"

    def __init__(
        self,
        permission_policy: PermissionPolicy,
        timeout: int = DEFAULT_TIMEOUT_SECONDS,
    ) -> None:
        self._policy = permission_policy
        self._timeout = timeout

    def schema(self) -> dict[str, Any]:
        return {
            "name": self.name,
            "description": (
                "Run a shell command from the workspace directory and return its "
                "combined stdout, stderr, and exit code. Use this to run tests, "
                "build steps, or inspect the environment. Execution is gated by a "
                "permission policy and may be denied."
            ),
            "input_schema": {
                "type": "object",
                "properties": {
                    "command": {
                        "type": "string",
                        "description": "The shell command to execute.",
                    }
                },
                "required": ["command"],
            },
        }

    def execute(self, tool_input: dict[str, Any], ws: Workspace) -> str:
        command = tool_input.get("command")
        if not isinstance(command, str) or command.strip() == "":
            raise ToolError("run_command requires a non-empty 'command'")

        if not self._policy.authorize(command):
            if self._policy.mode == "deny":
                raise ToolError(
                    "run_command is disabled by the 'deny' permission policy"
                )
            raise ToolError(f"command was not permitted by the operator: {command!r}")

        try:
            argv = shlex.split(command)
        except ValueError as exc:
            raise ToolError(f"could not parse command: {exc}") from exc
        if not argv:
            raise ToolError("empty command after parsing")

        try:
            proc = subprocess.run(
                argv,
                cwd=str(ws.root),
                capture_output=True,
                text=True,
                timeout=self._timeout,
                shell=False,
            )
        except FileNotFoundError as exc:
            raise ToolError(f"command not found: {argv[0]!r}") from exc
        except subprocess.TimeoutExpired as exc:
            partial = _truncate(_decode(exc.stdout) + _decode(exc.stderr))
            raise ToolError(
                f"command timed out after {self._timeout}s: {command!r}\n"
                f"--- partial output ---\n{partial}"
            ) from exc

        body = _truncate((proc.stdout or "") + (proc.stderr or ""))
        return f"exit code: {proc.returncode}\n--- output ---\n{body}"


def _decode(data: Any) -> str:
    if data is None:
        return ""
    if isinstance(data, bytes):
        return data.decode("utf-8", errors="replace")
    return str(data)


def _truncate(text: str) -> str:
    if len(text) <= MAX_OUTPUT_CHARS:
        return text
    return text[:MAX_OUTPUT_CHARS] + "\n... (output truncated)"
