"""Unit tests for the list_files tool."""

from __future__ import annotations

import pytest

from agent.errors import SandboxViolation, ToolError
from agent.tools.list_files import ListFilesTool


def _seed(workspace):
    (workspace.root / "a.txt").write_text("a")
    sub = workspace.root / "pkg"
    sub.mkdir()
    (sub / "b.py").write_text("b")
    (sub / "c.py").write_text("c")


def test_lists_nested_files_relative_to_root(workspace):
    _seed(workspace)
    result = ListFilesTool().execute({}, workspace)
    lines = set(result.splitlines())
    assert lines == {"a.txt", "pkg/b.py", "pkg/c.py"}


def test_lists_within_subdirectory(workspace):
    _seed(workspace)
    result = ListFilesTool().execute({"path": "pkg"}, workspace)
    assert set(result.splitlines()) == {"pkg/b.py", "pkg/c.py"}


def test_ignores_dot_git(workspace):
    _seed(workspace)
    git = workspace.root / ".git"
    git.mkdir()
    (git / "config").write_text("[core]")
    result = ListFilesTool().execute({}, workspace)
    assert ".git/config" not in result.splitlines()


def test_empty_directory_reports_no_files(workspace):
    result = ListFilesTool().execute({}, workspace)
    assert result == "(no files)"


def test_traversal_rejected(workspace):
    with pytest.raises(SandboxViolation):
        ListFilesTool().execute({"path": ".."}, workspace)


def test_missing_directory_raises(workspace):
    with pytest.raises(ToolError, match="not found"):
        ListFilesTool().execute({"path": "nope"}, workspace)
