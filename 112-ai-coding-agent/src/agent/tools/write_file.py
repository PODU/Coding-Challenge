"""write_file tool: create or overwrite a workspace-confined file."""

from __future__ import annotations

from typing import Any

from agent.errors import ToolError
from agent.workspace import Workspace

MAX_WRITE_BYTES = 1_000_000


class WriteFileTool:
    name = "write_file"

    def schema(self) -> dict[str, Any]:
        return {
            "name": self.name,
            "description": (
                "Create a new file or overwrite an existing one inside the "
                "workspace, writing the given text content. Parent directories are "
                "created automatically. The path is relative to the workspace root."
            ),
            "input_schema": {
                "type": "object",
                "properties": {
                    "path": {
                        "type": "string",
                        "description": "File path relative to the workspace root.",
                    },
                    "content": {
                        "type": "string",
                        "description": "Full text content to write to the file.",
                    },
                },
                "required": ["path", "content"],
            },
        }

    def execute(self, tool_input: dict[str, Any], ws: Workspace) -> str:
        path = tool_input.get("path")
        content = tool_input.get("content")
        if not isinstance(path, str) or path == "":
            raise ToolError("write_file requires a non-empty 'path'")
        if not isinstance(content, str):
            raise ToolError("write_file requires string 'content'")
        if len(content.encode("utf-8")) > MAX_WRITE_BYTES:
            raise ToolError(
                f"refusing to write more than {MAX_WRITE_BYTES} bytes to {path}"
            )

        target = ws.resolve(path)
        if target.is_dir():
            raise ToolError(f"path is a directory, not a file: {path}")

        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(content, encoding="utf-8")
        return f"Wrote {len(content)} characters to {path}"
