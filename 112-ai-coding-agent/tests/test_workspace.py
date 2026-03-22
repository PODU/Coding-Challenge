"""Path-confinement tests for the Workspace sandbox."""

from __future__ import annotations

import sys

import pytest

from agent.errors import SandboxViolation
from agent.workspace import Workspace


def test_resolve_accepts_in_root_paths(workspace):
    target = workspace.resolve("sub/dir/file.txt")
    assert target.is_relative_to(workspace.root)
    assert target.name == "file.txt"


def test_resolve_accepts_root_itself(workspace):
    assert workspace.resolve(".") == workspace.root


def test_resolve_rejects_parent_traversal(workspace):
    with pytest.raises(SandboxViolation):
        workspace.resolve("../escape.txt")


def test_resolve_rejects_deep_traversal(workspace):
    with pytest.raises(SandboxViolation):
        workspace.resolve("a/b/../../../escape.txt")


def test_resolve_rejects_absolute_path_outside(workspace, tmp_path):
    outside = tmp_path.parent / "outside.txt"
    with pytest.raises(SandboxViolation):
        workspace.resolve(str(outside))


@pytest.mark.skipif(
    sys.platform == "win32",
    reason="symlink creation typically needs privileges on Windows",
)
def test_resolve_rejects_symlink_escape(workspace, tmp_path):
    secret_dir = tmp_path.parent / "secret"
    secret_dir.mkdir()
    (secret_dir / "data.txt").write_text("top secret")

    link = workspace.root / "link"
    link.symlink_to(secret_dir)

    with pytest.raises(SandboxViolation):
        workspace.resolve("link/data.txt")


def test_missing_root_raises():
    with pytest.raises(FileNotFoundError):
        Workspace("/this/path/does/not/exist/anywhere")


def test_root_must_be_directory(tmp_path):
    f = tmp_path / "a_file.txt"
    f.write_text("x")
    with pytest.raises((ValueError, NotADirectoryError)):
        Workspace(f)
