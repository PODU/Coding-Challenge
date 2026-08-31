"""Human approval gate between planning and coding."""

from agentloop import state as st
from agentloop.backend import MockBackend
from agentloop.dispatcher import Dispatcher
from conftest import CALC_FULL, PLAN, backend_map_factory, init_run, make_cfg


def _dispatcher(run_dir, backend):
    return Dispatcher(run_dir,
                      backend_factory=backend_map_factory({"r1": backend}))


def test_run_blocks_until_approved(tmp_path):
    cfg = make_cfg(auto_approve=False)
    run_dir = init_run(tmp_path, cfg)
    backend = MockBackend([PLAN, CALC_FULL])
    state = _dispatcher(run_dir, backend).run()
    assert state.status == st.AWAITING_APPROVAL
    assert state.approved is False
    assert (run_dir / "plan.md").read_text() == PLAN
    # Coding must not have happened.
    assert backend.responses == [CALC_FULL]

    # Human approves, run resumes to completion.
    state.approved = True
    state.save(run_dir)
    final = _dispatcher(run_dir, backend).run()
    assert final.status == st.DONE


def test_rejection_via_cli(tmp_path, capsys):
    from agentloop.cli import main

    cfg = make_cfg(auto_approve=False)
    run_dir = init_run(tmp_path, cfg)
    state = st.RunState.load(run_dir)
    state.status = st.AWAITING_APPROVAL
    state.save(run_dir)

    rc = main(["--runs-dir", str(tmp_path / "runs"),
               "reject", "r1", "--reason", "wrong approach"])
    assert rc == 0
    final = st.RunState.load(run_dir)
    assert final.status == st.FAILED
    assert "wrong approach" in final.error


def test_approve_cli_requires_pending_state(tmp_path):
    import pytest

    from agentloop.cli import main

    cfg = make_cfg()
    run_dir = init_run(tmp_path, cfg)  # status: planning
    assert st.RunState.load(run_dir).status == st.PLANNING
    with pytest.raises(SystemExit, match="not awaiting approval"):
        main(["--runs-dir", str(tmp_path / "runs"), "approve", "r1"])
