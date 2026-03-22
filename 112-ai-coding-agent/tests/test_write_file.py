"""Unit tests for the write_file tool."""

from __future__ import annotations

import pytest

from agent.errors import SandboxViolation, ToolError
from agent.tools.write_file import WriteFileTool


def test_creates_file_with_exact_bytes(workspace):
    WriteFileTool().execute({"path": "out.txt", "content": "abc\n"}, workspace)
    written = (workspace.root / "out.txt").read_text(encoding="utf-8")
    assert written == "abc\n"


def test_creates_parent_directories(workspace):
    WriteFileTool().execute(
        {"path": "deep/nested/dir/file.txt", "content": "hi"}, workspace
    )
    target = workspace.root / "deep" / "nested" / "dir" / "file.txt"
    assert target.exists()
    assert target.read_text(encoding="utf-8") == "hi"


def test_overwrites_existing_file(workspace):
    (workspace.root / "f.txt").write_text("old", encoding="utf-8")
    WriteFileTool().execute({"path": "f.txt", "content": "new"}, workspace)
    assert (workspace.root / "f.txt").read_text(encoding="utf-8") == "new"


def test_traversal_path_rejected_and_nothing_written(workspace, tmp_path):
    escape_target = tmp_path.parent / "escape.txt"
    with pytest.raises(SandboxViolation):
        WriteFileTool().execute(
            {"path": "../escape.txt", "content": "pwned"}, workspace
        )
    assert not escape_target.exists()


def test_non_string_content_raises(workspace):
    with pytest.raises(ToolError):
        WriteFileTool().execute({"path": "f.txt", "content": 123}, workspace)


def test_missing_arguments_raise(workspace):
    with pytest.raises(ToolError):
        WriteFileTool().execute({"content": "x"}, workspace)
