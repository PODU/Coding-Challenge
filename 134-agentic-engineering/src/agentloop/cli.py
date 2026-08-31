"""The agentloop command line."""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from . import state as st
from . import timeline
from .config import RunConfig
from .dispatcher import Dispatcher
from .workspace import copy_project


def _echo(msg: str) -> None:
    print(msg)


def start_run(cfg: RunConfig, runs_dir: Path,
              stop_after: int | None = None) -> st.RunState:
    project = Path(cfg.project)
    if not project.is_dir():
        raise SystemExit(f"project directory not found: {project}")
    state = st.RunState.new(cfg)
    run_dir = runs_dir / state.run_id
    run_dir.mkdir(parents=True)
    copy_project(project, run_dir / "workspace")
    state.save(run_dir)
    return Dispatcher(run_dir, stop_after=stop_after, log=_echo).run()


def cmd_start(args: argparse.Namespace) -> int:
    cfg = RunConfig(
        task=args.task, project=args.project, backend=args.backend,
        mock_dir=args.mock_dir, gate=args.gate,
        gate_fail_times=args.gate_fail_times,
        sonar_host=args.sonar_host, sonar_token=args.sonar_token,
        sonar_project_key=args.sonar_project_key,
        max_iterations=args.max_iterations, candidates=args.candidates,
        auto_approve=args.auto_approve)
    if args.test_cmd:
        cfg.test_cmd = RunConfig.parse_cmd(args.test_cmd)
    state = start_run(cfg, Path(args.runs_dir), stop_after=args.stop_after)
    return _report(state)


def cmd_resume(args: argparse.Namespace) -> int:
    run_dir = Path(args.runs_dir) / args.run_id
    if not st.RunState.exists(run_dir):
        raise SystemExit(f"no such run: {args.run_id}")
    state = Dispatcher(run_dir, log=_echo).run()
    return _report(state)


def _report(state: st.RunState) -> int:
    print(f"\nrun {state.run_id}: {state.status}")
    if state.status == st.AWAITING_APPROVAL:
        print(f"plan is in runs/{state.run_id}/plan.md")
        print(f"approve with: agentloop approve {state.run_id}  "
              f"(then: agentloop resume {state.run_id})")
        return 0
    if state.winner:
        print(f"winner: {state.winner}")
    if state.error:
        print(f"error: {state.error}")
    return 0 if state.status == st.DONE else 1


def cmd_runs(args: argparse.Namespace) -> int:
    runs = st.list_runs(Path(args.runs_dir))
    if not runs:
        print("(no runs)")
        return 0
    print(f"{'RUN':<22}{'STATUS':<20}{'ITER':<6}{'UPDATED':<21}TASK")
    for r in runs:
        print(f"{r.run_id:<22}{r.status:<20}{r.iteration:<6}"
              f"{r.updated:<21}{r.task[:50]}")
    return 0


def _set_approval(args: argparse.Namespace, approve: bool) -> int:
    run_dir = Path(args.runs_dir) / args.run_id
    if not st.RunState.exists(run_dir):
        raise SystemExit(f"no such run: {args.run_id}")
    state = st.RunState.load(run_dir)
    if state.status != st.AWAITING_APPROVAL:
        raise SystemExit(f"run is not awaiting approval (status: {state.status})")
    if approve:
        state.approved = True
        state.save(run_dir)
        print(f"approved. continue with: agentloop resume {state.run_id}")
    else:
        state.status = st.FAILED
        state.error = f"plan rejected: {args.reason}"
        state.save(run_dir)
        st.append_transition(run_dir, "approval", st.AWAITING_APPROVAL,
                             st.FAILED, 0.0, state.error)
        print("rejected.")
    return 0


def cmd_approve(args: argparse.Namespace) -> int:
    return _set_approval(args, True)


def cmd_reject(args: argparse.Namespace) -> int:
    return _set_approval(args, False)


def cmd_timeline(args: argparse.Namespace) -> int:
    run_dir = Path(args.runs_dir) / args.run_id
    if not st.RunState.exists(run_dir):
        raise SystemExit(f"no such run: {args.run_id}")
    if args.mermaid:
        print(timeline.render_mermaid(run_dir))
    else:
        print(timeline.render_table(run_dir))
    return 0


def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(prog="agentloop",
                                description="Autonomous coding loop orchestrator")
    p.add_argument("--runs-dir", default="runs")
    sub = p.add_subparsers(dest="cmd", required=True)

    s = sub.add_parser("start", help="start a new run")
    s.add_argument("--task", required=True)
    s.add_argument("--project", required=True,
                   help="path to the project to operate on")
    s.add_argument("--backend", choices=["mock", "claude"], default="mock")
    s.add_argument("--mock-dir", help="directory of scripted agent responses")
    s.add_argument("--gate", choices=["none", "mock", "sonar"], default="none")
    s.add_argument("--gate-fail-times", type=int, default=0)
    s.add_argument("--sonar-host", default="http://localhost:9000")
    s.add_argument("--sonar-token", default="")
    s.add_argument("--sonar-project-key", default="")
    s.add_argument("--test-cmd", help="test command (default: python -m pytest -q)")
    s.add_argument("--max-iterations", type=int, default=3)
    s.add_argument("--candidates", type=int, default=1)
    s.add_argument("--auto-approve", action="store_true")
    s.add_argument("--stop-after", type=int, default=None,
                   help="stop after N transitions (crash simulation)")
    s.set_defaults(fn=cmd_start)

    r = sub.add_parser("resume", help="resume an interrupted run")
    r.add_argument("run_id")
    r.set_defaults(fn=cmd_resume)

    ls = sub.add_parser("runs", help="list past runs and their status")
    ls.set_defaults(fn=cmd_runs)

    a = sub.add_parser("approve", help="approve a pending plan")
    a.add_argument("run_id")
    a.set_defaults(fn=cmd_approve)

    rj = sub.add_parser("reject", help="reject a pending plan")
    rj.add_argument("run_id")
    rj.add_argument("--reason", default="rejected by operator")
    rj.set_defaults(fn=cmd_reject)

    t = sub.add_parser("timeline", help="show the run timeline")
    t.add_argument("run_id")
    t.add_argument("--mermaid", action="store_true")
    t.set_defaults(fn=cmd_timeline)
    return p


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    return args.fn(args)


if __name__ == "__main__":
    sys.exit(main())
