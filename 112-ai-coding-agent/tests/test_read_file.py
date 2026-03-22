"""Unit tests for the read_file tool."""

from __future__ import annotations

import pytest

from agent.errors import SandboxViolation, ToolError
from agent.tools.read_file import ReadFileTool


def test_reads_existing_file(workspace):
    (workspace.root / "hello.txt").write_text("hello world", encoding="utf-8")
    result = ReadFileTool().execute({"path": "hello.txt"}, workspace)
    assert result == "hello world"


def test_reads_nested_file(workspace):
    nested = workspace.root / "pkg" / "mod.py"
    nested.parent.mkdir(parents=True)
    nested.write_text("x = 1\n", encoding="utf-8")
    result = ReadFileTool().execute({"path": "pkg/mod.py"}, workspace)
    assert result == "x = 1\n"


def test_missing_file_raises_tool_error(workspace):
    with pytest.raises(ToolError, match="file not found"):
        ReadFileTool().execute({"path": "nope.txt"}, workspace)


def test_directory_path_raises(workspace):
    (workspace.root / "adir").mkdir()
    with pytest.raises(ToolError, match="directory"):
        ReadFileTool().execute({"path": "adir"}, workspace)


def test_traversal_path_rejected(workspace):
    with pytest.raises(SandboxViolation):
        ReadFileTool().execute({"path": "../outside.txt"}, workspace)


def test_missing_path_argument_raises(workspace):
    with pytest.raises(ToolError):
        ReadFileTool().execute({}, workspace)
