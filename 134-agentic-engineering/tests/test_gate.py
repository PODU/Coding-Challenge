"""Quality gate integration: gate violations route back through the fix loop."""

from agentloop import state as st
from agentloop.backend import MockBackend
from agentloop.dispatcher import Dispatcher
from agentloop.gate import GateResult, MockGate, NullGate
from conftest import CALC_FULL, PLAN, backend_map_factory, init_run, make_cfg


def test_null_gate_always_passes(tmp_path):
    assert NullGate().evaluate(tmp_path).passed is True


def test_mock_gate_fails_then_passes_and_counter_persists(tmp_path):
    gate = MockGate(fail_times=2, run_dir=tmp_path)
    assert gate.evaluate(tmp_path).passed is False
    # Fresh instance == process restart; counter comes from disk.
    gate2 = MockGate(fail_times=2, run_dir=tmp_path)
    assert gate2.evaluate(tmp_path).passed is False
    assert gate2.evaluate(tmp_path).passed is True


def test_gate_violation_routes_to_fix_then_passes(tmp_path):
    cfg = make_cfg(gate="mock", gate_fail_times=1)
    run_dir = init_run(tmp_path, cfg)
    # code passes tests immediately; gate fails once; fix re-emits; second pass OK
    backend = MockBackend([PLAN, CALC_FULL, CALC_FULL])
    final = Dispatcher(run_dir,
                       backend_factory=backend_map_factory({"r1": backend})).run()
    assert final.status == st.DONE
    assert final.gate_passed is True
    assert final.iteration == 1
    nodes = [t["node"] for t in st.read_transitions(run_dir)]
    assert nodes == ["planning", "awaiting_approval", "coding", "testing",
                     "reviewing", "fixing", "testing", "reviewing"]
    # Gate failure detail was handed to the fix node.
    assert "code_smells" in (run_dir / "last_failure.txt").read_text()


def test_gate_never_passing_hits_iteration_limit(tmp_path):
    cfg = make_cfg(gate="mock", gate_fail_times=99, max_iterations=2)
    run_dir = init_run(tmp_path, cfg)
    backend = MockBackend([PLAN, CALC_FULL, CALC_FULL, CALC_FULL])
    final = Dispatcher(run_dir,
                       backend_factory=backend_map_factory({"r1": backend})).run()
    assert final.status == st.FAILED
    assert "quality gate" in final.error


def test_gate_result_dataclass():
    r = GateResult(True, "ok")
    assert r.passed and r.detail == "ok"
