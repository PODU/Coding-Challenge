"""Fan-out: parallel candidates in isolated project copies, join picks winner."""

from agentloop import state as st
from agentloop.backend import MockBackend
from agentloop.dispatcher import Dispatcher
from conftest import (CALC_FULL, CALC_PARTIAL, PLAN, backend_map_factory,
                      init_run, make_cfg)


def test_fanout_two_candidates_winner_by_fewest_iterations(tmp_path):
    cfg = make_cfg(candidates=2)
    run_dir = init_run(tmp_path, cfg)
    backends = {
        "r1": MockBackend([PLAN]),
        "cand-1": MockBackend([CALC_PARTIAL, CALC_FULL]),  # needs 1 fix
        "cand-2": MockBackend([CALC_FULL]),                # correct first try
    }
    final = Dispatcher(run_dir,
                       backend_factory=backend_map_factory(backends)).run()
    assert final.status == st.DONE
    assert final.winner == "cand-2"
    # Winner's workspace copied out for the operator.
    winner_calc = (run_dir / "winner" / "calc.py").read_text()
    assert "if b == 0:" in winner_calc
    # Both candidates ran in isolated copies with their own state.
    c1 = st.RunState.load(run_dir / "candidates" / "cand-1")
    c2 = st.RunState.load(run_dir / "candidates" / "cand-2")
    assert c1.status == st.DONE and c1.iteration == 1
    assert c2.status == st.DONE and c2.iteration == 0
    nodes = [t["node"] for t in st.read_transitions(run_dir)]
    assert nodes == ["planning", "awaiting_approval", "fanout", "joining"]


def test_fanout_survives_one_failed_candidate(tmp_path):
    cfg = make_cfg(candidates=2, max_iterations=1)
    run_dir = init_run(tmp_path, cfg)
    backends = {
        "r1": MockBackend([PLAN]),
        "cand-1": MockBackend([CALC_PARTIAL, CALC_PARTIAL]),  # never fixes it
        "cand-2": MockBackend([CALC_FULL]),
    }
    final = Dispatcher(run_dir,
                       backend_factory=backend_map_factory(backends)).run()
    assert final.status == st.DONE
    assert final.winner == "cand-2"
    assert st.RunState.load(run_dir / "candidates" / "cand-1").status == st.FAILED


def test_all_candidates_failing_fails_run(tmp_path):
    cfg = make_cfg(candidates=2, max_iterations=1)
    run_dir = init_run(tmp_path, cfg)
    backends = {
        "r1": MockBackend([PLAN]),
        "cand-1": MockBackend([CALC_PARTIAL, CALC_PARTIAL]),
        "cand-2": MockBackend([CALC_PARTIAL, CALC_PARTIAL]),
    }
    final = Dispatcher(run_dir,
                       backend_factory=backend_map_factory(backends)).run()
    assert final.status == st.FAILED
    assert "all candidates failed" in final.error


def test_fanout_crash_recovery_reuses_existing_candidates(tmp_path):
    cfg = make_cfg(candidates=2)
    run_dir = init_run(tmp_path, cfg)
    backends = {
        "r1": MockBackend([PLAN]),
        "cand-1": MockBackend([CALC_FULL]),
        "cand-2": MockBackend([CALC_FULL]),
    }
    factory = backend_map_factory(backends)
    # Crash right after the fanout transition completes.
    crashed = Dispatcher(run_dir, backend_factory=factory, stop_after=3).run()
    assert crashed.status == st.JOINING
    # Resume: fanout is done, join runs; candidate states must not be re-created.
    final = Dispatcher(run_dir, backend_factory=factory).run()
    assert final.status == st.DONE
    assert final.winner in ("cand-1", "cand-2")
