"""list_files tool: recursively list files relative to the workspace."""

from __future__ import annotations

from typing import Any

from agent.errors import ToolError
from agent.workspace import Workspace

# Directories that are noise for a coding agent and slow to walk.
_IGNORED_DIR_NAMES = {".git", "__pycache__", ".pytest_cache", ".venv", "venv"}

MAX_ENTRIES = 5000


class ListFilesTool:
    name = "list_files"

    def schema(self) -> dict[str, Any]:
        return {
            "name": self.name,
            "description": (
                "Recursively list files under a directory in the workspace, as "
                "paths relative to the workspace root. Call this to discover the "
                "project structure. Omit 'path' to list from the workspace root."
            ),
            "input_schema": {
                "type": "object",
                "properties": {
                    "path": {
                        "type": "string",
                        "description": (
                            "Directory to list, relative to the workspace root. "
                            "Defaults to the root."
                        ),
                    }
                },
                "required": [],
            },
        }

    def execute(self, tool_input: dict[str, Any], ws: Workspace) -> str:
        rel = tool_input.get("path", "") or ""
        if not isinstance(rel, str):
            raise ToolError("list_files 'path' must be a string")

        base = ws.resolve(rel)
        if not base.exists():
            raise ToolError(f"directory not found: {rel}")
        if not base.is_dir():
            raise ToolError(f"path is not a directory: {rel}")

        results: list[str] = []
        for entry in sorted(base.rglob("*")):
            if entry.is_dir():
                continue
            if _IGNORED_DIR_NAMES.intersection(entry.parts):
                continue
            results.append(entry.relative_to(ws.root).as_posix())
            if len(results) >= MAX_ENTRIES:
                results.append(f"... (truncated at {MAX_ENTRIES} entries)")
                break

        if not results:
            return "(no files)"
        return "\n".join(results)
