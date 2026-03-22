"""Unit tests for the run_command tool and its permission gate.

Commands use ``python`` (guaranteed on PATH for this challenge) rather than
absolute interpreter paths, so the argv split stays free of backslashes.
"""

from __future__ import annotations

import pytest

from agent.errors import ToolError
from agent.permissions import PermissionPolicy
from agent.tools.run_command import RunCommandTool


def test_allow_runs_and_captures_stdout_and_exit_code(workspace):
    tool = RunCommandTool(PermissionPolicy(mode="allow"))
    result = tool.execute({"command": 'python -c "print(7 * 6)"'}, workspace)
    assert "42" in result
    assert "exit code: 0" in result


def test_allow_reports_nonzero_exit(workspace):
    tool = RunCommandTool(PermissionPolicy(mode="allow"))
    result = tool.execute({"command": 'python -c "import sys; sys.exit(3)"'}, workspace)
    assert "exit code: 3" in result


def test_deny_blocks_execution(workspace):
    tool = RunCommandTool(PermissionPolicy(mode="deny"))
    with pytest.raises(ToolError, match="deny"):
        tool.execute({"command": 'python -c "print(1)"'}, workspace)


def test_prompt_accepts_when_confirmer_says_yes(workspace):
    policy = PermissionPolicy(mode="prompt", confirmer=lambda cmd: True)
    tool = RunCommandTool(policy)
    result = tool.execute({"command": 'python -c "print(123)"'}, workspace)
    assert "123" in result


def test_prompt_declines_when_confirmer_says_no(workspace):
    policy = PermissionPolicy(mode="prompt", confirmer=lambda cmd: False)
    tool = RunCommandTool(policy)
    with pytest.raises(ToolError, match="not permitted"):
        tool.execute({"command": 'python -c "print(1)"'}, workspace)


def test_runs_in_workspace_cwd(workspace):
    tool = RunCommandTool(PermissionPolicy(mode="allow"))
    tool.execute(
        {"command": "python -c \"open('made.txt','w').write('x')\""}, workspace
    )
    assert (workspace.root / "made.txt").read_text() == "x"


def test_timeout_kills_process(workspace):
    tool = RunCommandTool(PermissionPolicy(mode="allow"), timeout=1)
    with pytest.raises(ToolError, match="timed out"):
        tool.execute(
            {"command": 'python -c "import time; time.sleep(10)"'}, workspace
        )


def test_empty_command_raises(workspace):
    tool = RunCommandTool(PermissionPolicy(mode="allow"))
    with pytest.raises(ToolError):
        tool.execute({"command": "   "}, workspace)


def test_invalid_permission_mode_rejected():
    with pytest.raises(ValueError):
        PermissionPolicy(mode="sometimes")
