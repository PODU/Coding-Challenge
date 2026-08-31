"""Graph nodes. Each node does one unit of work and names the next status.

Nodes are pure functions of the on-disk state plus their injected
backend/gate, so a crashed node can simply be re-run: it rewrites the same
plan/files/outputs and the dispatcher carries on.
"""

from __future__ import annotations

import subprocess
from concurrent.futures import ThreadPoolExecutor
from dataclasses import dataclass
from pathlib import Path
from typing import Callable

from . import state as st
from .backend import AgentBackend
from .config import RunConfig
from .gate import QualityGate
from .workspace import (copy_project, file_listing, parse_file_blocks,
                        read_sources, write_files)


class NodeError(RuntimeError):
    pass


@dataclass
class Ctx:
    run_dir: Path
    workspace: Path
    state: st.RunState
    cfg: RunConfig
    backend: AgentBackend
    gate: QualityGate
    backend_factory: Callable
    gate_factory: Callable


def _artifacts(ctx: Ctx) -> Path:
    d = ctx.run_dir / "artifacts"
    d.mkdir(exist_ok=True)
    return d


PLAN_PROMPT = """\
You are the planning node of an autonomous coding loop.

Task:
{task}

Project files:
{listing}

Produce a short numbered implementation plan (which files to change and how).
Plain markdown only — do not write any code yet.
"""


def plan_node(ctx: Ctx) -> tuple[str, str]:
    prompt = PLAN_PROMPT.format(task=ctx.cfg.task,
                                listing=file_listing(ctx.workspace))
    plan = ctx.backend.invoke(prompt)
    (ctx.run_dir / st.PLAN_FILE).write_text(plan, encoding="utf-8")
    if ctx.cfg.auto_approve:
        ctx.state.approved = True
        return st.AWAITING_APPROVAL, "plan written; auto-approved"
    return st.AWAITING_APPROVAL, "plan written; awaiting human approval"


def approval_node(ctx: Ctx) -> tuple[str, str]:
    """Only reached once state.approved is True (the dispatcher blocks otherwise)."""
    if ctx.cfg.candidates > 1:
        return st.FANOUT, f"approved; fanning out {ctx.cfg.candidates} candidates"
    return st.CODING, "approved"


CODE_PROMPT = """\
You are the coding node of an autonomous coding loop.

Task:
{task}

Approved plan:
{plan}

Current project sources:
{sources}

Implement the plan. Return every file you change, as complete file contents,
in exactly this format (no other prose is needed):

=== FILE: relative/path ===
<full file content>
=== END FILE ===
"""

FIX_PROMPT = """\
You are the fix node of an autonomous coding loop. A quality check failed.

Task:
{task}

Failure output:
{failure}

Current project sources:
{sources}

Fix the failure. Return every file you change, as complete file contents,
in exactly this format:

=== FILE: relative/path ===
<full file content>
=== END FILE ===
"""


def _apply_agent_files(ctx: Ctx, response: str, label: str) -> str:
    files = parse_file_blocks(response)
    if not files:
        raise NodeError(f"{label} node: agent returned no FILE blocks")
    return ", ".join(write_files(ctx.workspace, files))


def code_node(ctx: Ctx) -> tuple[str, str]:
    plan = (ctx.run_dir / st.PLAN_FILE).read_text(encoding="utf-8")
    prompt = CODE_PROMPT.format(task=ctx.cfg.task, plan=plan,
                                sources=read_sources(ctx.workspace))
    written = _apply_agent_files(ctx, ctx.backend.invoke(prompt), "code")
    return st.TESTING, f"wrote {written}"


def fix_node(ctx: Ctx) -> tuple[str, str]:
    ctx.state.iteration += 1
    failure = (ctx.run_dir / st.FAILURE_FILE).read_text(encoding="utf-8")
    prompt = FIX_PROMPT.format(task=ctx.cfg.task, failure=failure,
                               sources=read_sources(ctx.workspace))
    written = _apply_agent_files(ctx, ctx.backend.invoke(prompt), "fix")
    return st.TESTING, f"iteration {ctx.state.iteration}: rewrote {written}"


def _route_failure(ctx: Ctx, kind: str, output: str) -> tuple[str, str]:
    (ctx.run_dir / st.FAILURE_FILE).write_text(output, encoding="utf-8")
    if ctx.state.iteration >= ctx.cfg.max_iterations:
        ctx.state.error = (f"{kind} still failing after "
                           f"{ctx.state.iteration} fix iterations")
        return st.FAILED, ctx.state.error
    return st.FIXING, f"{kind} failed; routing to fix node"


def test_node(ctx: Ctx) -> tuple[str, str]:
    try:
        proc = subprocess.run(ctx.cfg.test_cmd, cwd=ctx.workspace,
                              capture_output=True, text=True, timeout=600)
    except (FileNotFoundError, subprocess.TimeoutExpired) as e:
        raise NodeError(f"test command failed to run: {e}") from e
    output = (proc.stdout or "") + (proc.stderr or "")
    art = _artifacts(ctx)
    n = len(list(art.glob("test_*.txt"))) + 1
    (art / f"test_{n:02d}.txt").write_text(output, encoding="utf-8")
    if proc.returncode == 0:
        ctx.state.tests_passed = True
        return st.REVIEWING, "tests passed"
    ctx.state.tests_passed = False
    return _route_failure(ctx, "tests", output)


def review_node(ctx: Ctx) -> tuple[str, str]:
    result = ctx.gate.evaluate(ctx.workspace)
    (_artifacts(ctx) / "gate_result.txt").write_text(
        f"passed={result.passed}\n{result.detail}\n", encoding="utf-8")
    if result.passed:
        ctx.state.gate_passed = True
        return st.DONE, f"quality gate passed: {result.detail}"
    ctx.state.gate_passed = False
    return _route_failure(ctx, "quality gate", result.detail)


def candidate_dirs(run_dir: Path, n: int) -> list[Path]:
    return [run_dir / "candidates" / f"cand-{i + 1}" for i in range(n)]


def fanout_node(ctx: Ctx) -> tuple[str, str]:
    from .dispatcher import Dispatcher  # cycle: dispatcher imports nodes

    dirs = candidate_dirs(ctx.run_dir, ctx.cfg.candidates)
    for d in dirs:
        if st.RunState.exists(d):
            continue  # set up before a crash; don't clobber its progress
        d.mkdir(parents=True, exist_ok=True)
        copy_project(ctx.workspace, d / "workspace")
        plan = ctx.run_dir / st.PLAN_FILE
        if plan.is_file():
            (d / st.PLAN_FILE).write_text(plan.read_text(encoding="utf-8"),
                                          encoding="utf-8")
        cfg = RunConfig.from_dict({**ctx.cfg.to_dict(),
                                   "candidates": 1, "auto_approve": True})
        cand = st.RunState.new(cfg, run_id=d.name, status=st.CODING)
        cand.approved = True
        cand.save(d)

    def run_candidate(d: Path) -> str:
        return Dispatcher(d, backend_factory=ctx.backend_factory,
                          gate_factory=ctx.gate_factory).run().status

    with ThreadPoolExecutor(max_workers=len(dirs)) as pool:
        results = list(pool.map(run_candidate, dirs))
    summary = ", ".join(f"{d.name}={s}" for d, s in zip(dirs, results))
    return st.JOINING, summary


def _score(d: Path) -> tuple:
    """Higher is better: done > gate > tests > fewer fix iterations > faster."""
    cand = st.RunState.load(d)
    duration = sum(t["duration_s"] for t in st.read_transitions(d))
    return (cand.status == st.DONE, bool(cand.gate_passed),
            bool(cand.tests_passed), -cand.iteration, -duration)


def join_node(ctx: Ctx) -> tuple[str, str]:
    dirs = [d for d in candidate_dirs(ctx.run_dir, ctx.cfg.candidates)
            if st.RunState.exists(d)]
    if not dirs:
        ctx.state.error = "no candidates to join"
        return st.FAILED, ctx.state.error
    finished = [d for d in dirs if st.RunState.load(d).status == st.DONE]
    if not finished:
        ctx.state.error = "all candidates failed"
        return st.FAILED, ctx.state.error
    winner = max(finished, key=_score)
    win_state = st.RunState.load(winner)
    copy_project(winner / "workspace", ctx.run_dir / "winner")
    ctx.state.winner = winner.name
    ctx.state.tests_passed = win_state.tests_passed
    ctx.state.gate_passed = win_state.gate_passed
    return st.DONE, (f"winner {winner.name} "
                     f"(iterations={win_state.iteration}); "
                     f"result copied to winner/")


HANDLERS = {
    st.PLANNING: plan_node,
    st.AWAITING_APPROVAL: approval_node,
    st.CODING: code_node,
    st.TESTING: test_node,
    st.FIXING: fix_node,
    st.REVIEWING: review_node,
    st.FANOUT: fanout_node,
    st.JOINING: join_node,
}
