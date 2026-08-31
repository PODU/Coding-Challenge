"""End-to-end through the CLI with the on-disk mock session (fully offline)."""

import re

from agentloop import state as st
from agentloop.cli import main
from conftest import FIXTURE, MOCK_SESSION


def _extract_run_id(out: str) -> str:
    m = re.search(r"run (\S+):", out)
    assert m, out
    return m.group(1)


def test_cli_full_run_with_mock_session(tmp_path, capsys):
    runs_dir = tmp_path / "runs"
    rc = main(["--runs-dir", str(runs_dir), "start",
               "--task", "fix the calculator bugs",
               "--project", str(FIXTURE),
               "--backend", "mock", "--mock-dir", str(MOCK_SESSION),
               "--auto-approve"])
    out = capsys.readouterr().out
    assert rc == 0
    run_id = _extract_run_id(out)
    state = st.RunState.load(runs_dir / run_id)
    assert state.status == st.DONE
    assert state.iteration == 1  # 02_code leaves the divide bug; 03_fix fixes it
    fixed = (runs_dir / run_id / "workspace" / "calc.py").read_text()
    assert "if b == 0:" in fixed

    # runs manifest
    rc = main(["--runs-dir", str(runs_dir), "runs"])
    out = capsys.readouterr().out
    assert rc == 0
    assert run_id in out and "done" in out

    # timeline
    rc = main(["--runs-dir", str(runs_dir), "timeline", run_id])
    out = capsys.readouterr().out
    assert rc == 0
    for node in ("planning", "coding", "testing", "fixing", "reviewing"):
        assert node in out
    assert "transitions" in out

    rc = main(["--runs-dir", str(runs_dir), "timeline", run_id, "--mermaid"])
    out = capsys.readouterr().out
    assert out.startswith("gantt")


def test_cli_approval_flow(tmp_path, capsys):
    runs_dir = tmp_path / "runs"
    rc = main(["--runs-dir", str(runs_dir), "start",
               "--task", "fix the calculator bugs",
               "--project", str(FIXTURE),
               "--backend", "mock", "--mock-dir", str(MOCK_SESSION)])
    out = capsys.readouterr().out
    assert rc == 0
    run_id = _extract_run_id(out)
    assert st.RunState.load(runs_dir / run_id).status == st.AWAITING_APPROVAL
    assert "approve" in out

    rc = main(["--runs-dir", str(runs_dir), "approve", run_id])
    capsys.readouterr()
    assert rc == 0

    rc = main(["--runs-dir", str(runs_dir), "resume", run_id])
    capsys.readouterr()
    assert rc == 0
    assert st.RunState.load(runs_dir / run_id).status == st.DONE


def test_cli_crash_and_resume(tmp_path, capsys):
    runs_dir = tmp_path / "runs"
    main(["--runs-dir", str(runs_dir), "start",
          "--task", "fix the calculator bugs",
          "--project", str(FIXTURE),
          "--backend", "mock", "--mock-dir", str(MOCK_SESSION),
          "--auto-approve", "--stop-after", "3"])
    out = capsys.readouterr().out
    run_id = _extract_run_id(out)
    mid = st.RunState.load(runs_dir / run_id)
    assert mid.status not in st.TERMINAL

    rc = main(["--runs-dir", str(runs_dir), "resume", run_id])
    capsys.readouterr()
    assert rc == 0
    assert st.RunState.load(runs_dir / run_id).status == st.DONE
