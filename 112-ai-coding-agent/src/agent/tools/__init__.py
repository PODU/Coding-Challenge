"""The tool registry: collects tool schemas and dispatches tool calls."""

from __future__ import annotations

from typing import Any, Protocol, runtime_checkable

from agent.errors import ToolError
from agent.workspace import Workspace


@runtime_checkable
class Tool(Protocol):
    name: str

    def schema(self) -> dict[str, Any]:
        """Return the Anthropic tool definition (name, description, input_schema)."""
        ...

    def execute(self, tool_input: dict[str, Any], ws: Workspace) -> str:
        """Run the tool and return a text result, or raise ``ToolError``."""
        ...


class ToolRegistry:
    def __init__(self, tools: list[Tool]) -> None:
        self._by_name: dict[str, Tool] = {}
        for tool in tools:
            if tool.name in self._by_name:
                raise ValueError(f"duplicate tool name: {tool.name!r}")
            self._by_name[tool.name] = tool

    def names(self) -> list[str]:
        return list(self._by_name)

    def schemas(self) -> list[dict[str, Any]]:
        return [tool.schema() for tool in self._by_name.values()]

    def dispatch(
        self,
        name: str,
        tool_input: dict[str, Any],
        ws: Workspace,
    ) -> tuple[str, bool]:
        """Run a tool call and return ``(result_text, is_error)``.

        Unknown tools and ToolErrors become error results for the model rather
        than exceptions.
        """
        tool = self._by_name.get(name)
        if tool is None:
            return (f"Unknown tool: {name}", True)
        if not isinstance(tool_input, dict):
            return (f"Tool input for {name!r} must be an object", True)
        try:
            return (tool.execute(tool_input, ws), False)
        except ToolError as exc:
            return (str(exc), True)


def default_registry(permission_policy: Any | None = None) -> ToolRegistry:
    """Build a registry with the four standard tools. ``permission_policy``
    governs run_command and defaults to prompt mode."""
    # Imported here to avoid a circular import.
    from agent.permissions import PermissionPolicy
    from agent.tools.list_files import ListFilesTool
    from agent.tools.read_file import ReadFileTool
    from agent.tools.run_command import RunCommandTool
    from agent.tools.write_file import WriteFileTool

    policy = permission_policy or PermissionPolicy()
    return ToolRegistry(
        [
            ReadFileTool(),
            WriteFileTool(),
            ListFilesTool(),
            RunCommandTool(policy),
        ]
    )


__all__ = ["Tool", "ToolRegistry", "default_registry"]
