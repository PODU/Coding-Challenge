"""Core loop: plan -> code -> test -> fix -> test -> review -> done."""

from agentloop import state as st
from agentloop.backend import MockBackend
from agentloop.dispatcher import Dispatcher
from conftest import (CALC_FULL, CALC_PARTIAL, PLAN, backend_map_factory,
                      init_run, make_cfg)


def run_with(tmp_path, cfg, responses, **disp_kwargs):
    run_dir = init_run(tmp_path, cfg)
    backend = MockBackend(responses)
    disp = Dispatcher(run_dir, backend_factory=backend_map_factory({"r1": backend}),
                      **disp_kwargs)
    return run_dir, disp.run()


def test_happy_path_with_one_fix_iteration(tmp_path, cfg):
    run_dir, final = run_with(tmp_path, cfg, [PLAN, CALC_PARTIAL, CALC_FULL])
    assert final.status == st.DONE
    assert final.iteration == 1
    assert final.tests_passed is True
    assert final.gate_passed is True  # NullGate
    nodes = [t["node"] for t in st.read_transitions(run_dir)]
    assert nodes == ["planning", "awaiting_approval", "coding", "testing",
                     "fixing", "testing", "reviewing"]
    assert (run_dir / "plan.md").read_text() == PLAN
    assert (run_dir / "artifacts" / "test_01.txt").exists()
    assert (run_dir / "artifacts" / "test_02.txt").exists()


def test_immediately_correct_code_skips_fix(tmp_path, cfg):
    run_dir, final = run_with(tmp_path, cfg, [PLAN, CALC_FULL])
    assert final.status == st.DONE
    assert final.iteration == 0
    nodes = [t["node"] for t in st.read_transitions(run_dir)]
    assert "fixing" not in nodes


def test_iteration_limit_terminates_loop(tmp_path):
    cfg = make_cfg(max_iterations=2)
    # Code and every fix leave the divide bug in place.
    responses = [PLAN, CALC_PARTIAL, CALC_PARTIAL, CALC_PARTIAL]
    run_dir, final = run_with(tmp_path, cfg, responses)
    assert final.status == st.FAILED
    assert final.iteration == 2
    assert "still failing after 2 fix iterations" in final.error
    assert (run_dir / "last_failure.txt").exists()


def test_agent_without_file_blocks_fails_run(tmp_path, cfg):
    run_dir, final = run_with(tmp_path, cfg, [PLAN, "sorry, no code"])
    assert final.status == st.FAILED
    assert "no FILE blocks" in final.error


def test_state_persisted_after_every_transition(tmp_path, cfg):
    run_dir, final = run_with(tmp_path, cfg, [PLAN, CALC_FULL])
    on_disk = st.RunState.load(run_dir)
    assert on_disk.status == final.status == st.DONE
    assert on_disk.config["task"] == cfg.task
