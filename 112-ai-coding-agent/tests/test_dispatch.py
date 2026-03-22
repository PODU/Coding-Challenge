"""Registry dispatch tests: routing, unknown tools, and error conversion."""

from __future__ import annotations

from agent.errors import ToolError
from agent.tools import ToolRegistry


class _EchoTool:
    name = "echo"

    def schema(self):
        return {"name": self.name, "description": "echo", "input_schema": {}}

    def execute(self, tool_input, ws):
        return f"echo: {tool_input.get('text', '')}"


class _BoomTool:
    name = "boom"

    def schema(self):
        return {"name": self.name, "description": "boom", "input_schema": {}}

    def execute(self, tool_input, ws):
        raise ToolError("kaboom")


def test_routes_to_correct_handler(workspace):
    reg = ToolRegistry([_EchoTool(), _BoomTool()])
    result, is_error = reg.dispatch("echo", {"text": "hi"}, workspace)
    assert result == "echo: hi"
    assert is_error is False


def test_unknown_tool_is_error_not_exception(workspace):
    reg = ToolRegistry([_EchoTool()])
    result, is_error = reg.dispatch("missing", {}, workspace)
    assert is_error is True
    assert "Unknown tool" in result


def test_tool_error_becomes_error_result(workspace):
    reg = ToolRegistry([_BoomTool()])
    result, is_error = reg.dispatch("boom", {}, workspace)
    assert is_error is True
    assert result == "kaboom"


def test_schemas_exported_in_order():
    reg = ToolRegistry([_EchoTool(), _BoomTool()])
    names = [s["name"] for s in reg.schemas()]
    assert names == ["echo", "boom"]


def test_default_registry_has_four_tools():
    from agent.tools import default_registry

    reg = default_registry()
    assert set(reg.names()) == {
        "read_file",
        "write_file",
        "list_files",
        "run_command",
    }


def test_duplicate_tool_names_rejected():
    import pytest

    with pytest.raises(ValueError):
        ToolRegistry([_EchoTool(), _EchoTool()])
