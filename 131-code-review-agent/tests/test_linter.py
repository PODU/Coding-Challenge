import json

import pytest

from reviewagent.linter import (
    LintFinding,
    clone_at_commit,
    format_findings,
    parse_ruff_json,
    run_ruff,
)

RUFF_OUTPUT = json.dumps([
    {"filename": "app/orders.py", "code": "F841",
     "message": "Local variable `total` is assigned to but never used",
     "location": {"row": 12, "column": 5}},
    {"filename": "app/report.py", "code": "E722",
     "message": "Do not use bare `except`",
     "location": {"row": 88, "column": 1}},
])


def test_parse_ruff_json():
    findings = parse_ruff_json(RUFF_OUTPUT)
    assert findings == [
        LintFinding("app/orders.py", 12, "F841",
                    "Local variable `total` is assigned to but never used"),
        LintFinding("app/report.py", 88, "E722", "Do not use bare `except`"),
    ]


def test_parse_empty_output():
    assert parse_ruff_json("") == []
    assert parse_ruff_json("[]") == []


def test_format_findings_has_precise_locations():
    text = format_findings(parse_ruff_json(RUFF_OUTPUT))
    assert "app/orders.py:12 [F841]" in text
    assert "app/report.py:88 [E722]" in text


def test_format_findings_strips_repo_prefix():
    findings = [LintFinding("/tmp/checkout/app/x.py", 1, "E1", "m")]
    assert format_findings(findings, repo_prefix="/tmp/checkout") == \
        "- app/x.py:1 [E1] m"


class FakeProc:
    def __init__(self, returncode=0, stdout="[]", stderr=""):
        self.returncode = returncode
        self.stdout = stdout
        self.stderr = stderr


def test_run_ruff_success():
    calls = []

    def runner(cmd, **kwargs):
        calls.append(cmd)
        return FakeProc(stdout=RUFF_OUTPUT)

    findings = run_ruff("/repo", runner=runner)
    assert len(findings) == 2
    assert "--exit-zero" in calls[0]  # findings are data, not failure


def test_run_ruff_real_failure_raises():
    def runner(cmd, **kwargs):
        return FakeProc(returncode=2, stderr="ruff: config error")

    with pytest.raises(RuntimeError, match="config error"):
        run_ruff("/repo", runner=runner)


def test_clone_at_commit_command_sequence():
    cmds = []

    def runner(cmd, **kwargs):
        cmds.append(" ".join(cmd))
        return FakeProc()

    clone_at_commit("https://x/y.git", "abc123", "/dest", runner=runner)
    assert any("fetch" in c and "abc123" in c and "--depth 1" in c for c in cmds)
    assert any("checkout" in c and "FETCH_HEAD" in c for c in cmds)
