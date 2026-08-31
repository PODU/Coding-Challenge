"""Crash recovery: kill the process at arbitrary transition boundaries,
resume from the persisted state, and finish the run."""

import pytest

from agentloop import state as st
from agentloop.backend import DirMockBackend, MockBackend
from agentloop.dispatcher import Dispatcher
from conftest import (CALC_FULL, CALC_PARTIAL, MOCK_SESSION, PLAN,
                      backend_map_factory, init_run, make_cfg)


@pytest.mark.parametrize("crash_after", [1, 2, 3, 4, 5, 6])
def test_resume_after_crash_at_any_boundary(tmp_path, crash_after):
    cfg = make_cfg()
    run_dir = init_run(tmp_path, cfg)
    responses = [PLAN, CALC_PARTIAL, CALC_FULL]
    backend = MockBackend(list(responses))
    factory = backend_map_factory({"r1": backend})

    crashed = Dispatcher(run_dir, backend_factory=factory,
                         stop_after=crash_after).run()
    assert crashed.status not in st.TERMINAL

    # New process: reload purely from disk, feed the not-yet-consumed script.
    resumed_backend = MockBackend(responses[len(backend.calls):])
    final = Dispatcher(run_dir,
                       backend_factory=backend_map_factory(
                           {"r1": resumed_backend})).run()
    assert final.status == st.DONE
    nodes = [t["node"] for t in st.read_transitions(run_dir)]
    assert nodes == ["planning", "awaiting_approval", "coding", "testing",
                     "fixing", "testing", "reviewing"]


def test_dir_mock_backend_cursor_survives_restart(tmp_path):
    cfg = make_cfg(mock_dir=str(MOCK_SESSION))
    run_dir = init_run(tmp_path, cfg)

    # First "process" consumes only the plan response, then dies.
    b1 = DirMockBackend(MOCK_SESSION, run_dir)
    assert "Plan" in b1.invoke("plan please")
    # Second "process" resumes at response 2.
    b2 = DirMockBackend(MOCK_SESSION, run_dir)
    assert "subtract" in b2.invoke("code please")
    assert (run_dir / "mock_cursor").read_text() == "2"


def test_runs_manifest_lists_all_runs(tmp_path):
    cfg = make_cfg()
    init_run(tmp_path, cfg, run_id="run-a")
    init_run(tmp_path, cfg, run_id="run-b")
    runs = st.list_runs(tmp_path / "runs")
    assert [r.run_id for r in runs] == ["run-b", "run-a"]
    assert all(r.status == st.PLANNING for r in runs)
