"""read_file tool: return the contents of a workspace-confined file."""

from __future__ import annotations

from typing import Any

from agent.errors import ToolError
from agent.workspace import Workspace

# Reject reads larger than this so a giant file cannot blow up the context window.
MAX_READ_BYTES = 1_000_000


class ReadFileTool:
    name = "read_file"

    def schema(self) -> dict[str, Any]:
        return {
            "name": self.name,
            "description": (
                "Read and return the full contents of a text file inside the "
                "workspace. Call this to inspect an existing file before editing "
                "it. The path is relative to the workspace root."
            ),
            "input_schema": {
                "type": "object",
                "properties": {
                    "path": {
                        "type": "string",
                        "description": "File path relative to the workspace root.",
                    }
                },
                "required": ["path"],
            },
        }

    def execute(self, tool_input: dict[str, Any], ws: Workspace) -> str:
        path = tool_input.get("path")
        if not isinstance(path, str) or path == "":
            raise ToolError("read_file requires a non-empty 'path'")

        target = ws.resolve(path)
        if not target.exists():
            raise ToolError(f"file not found: {path}")
        if target.is_dir():
            raise ToolError(f"path is a directory, not a file: {path}")

        size = target.stat().st_size
        if size > MAX_READ_BYTES:
            raise ToolError(
                f"file too large to read ({size} bytes > {MAX_READ_BYTES} limit): {path}"
            )

        try:
            return target.read_text(encoding="utf-8")
        except UnicodeDecodeError as exc:
            raise ToolError(f"file is not valid UTF-8 text: {path}") from exc
