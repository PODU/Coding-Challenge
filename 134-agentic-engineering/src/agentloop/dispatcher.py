"""The main loop: load the state, run the node for its status, save, repeat.

Everything goes through files, so if the process dies only the node that was
running has to be redone.
"""

from __future__ import annotations

from pathlib import Path
from time import monotonic
from typing import Callable

from . import state as st
from .backend import make_backend
from .gate import make_gate
from .nodes import HANDLERS, Ctx, NodeError


class Dispatcher:
    def __init__(self, run_dir: Path,
                 backend_factory: Callable = make_backend,
                 gate_factory: Callable = make_gate,
                 stop_after: int | None = None,
                 log: Callable[[str], None] | None = None):
        self.run_dir = Path(run_dir)
        self.backend_factory = backend_factory
        self.gate_factory = gate_factory
        self.stop_after = stop_after  # stop early, as if crashed (for tests)
        self.log = log or (lambda msg: None)

    @property
    def workspace(self) -> Path:
        return self.run_dir / "workspace"

    def run(self) -> st.RunState:
        transitions = 0
        while True:
            state = st.RunState.load(self.run_dir)
            if state.status in st.TERMINAL:
                return state
            if state.status == st.AWAITING_APPROVAL and not state.approved:
                self.log(f"[{state.run_id}] blocked: awaiting approval "
                         f"(agentloop approve {state.run_id})")
                return state

            cfg = state.cfg
            ctx = Ctx(run_dir=self.run_dir, workspace=self.workspace,
                      state=state, cfg=cfg,
                      backend=self.backend_factory(cfg, self.run_dir),
                      gate=self.gate_factory(cfg, self.run_dir),
                      backend_factory=self.backend_factory,
                      gate_factory=self.gate_factory)

            node = state.status
            t0 = monotonic()
            try:
                next_status, detail = HANDLERS[node](ctx)
            except NodeError as e:
                next_status, detail = st.FAILED, str(e)
                state.error = str(e)
            duration = monotonic() - t0

            frm = state.status
            state.status = next_status
            state.save(self.run_dir)
            st.append_transition(self.run_dir, node, frm, next_status,
                                 duration, detail)
            self.log(f"[{state.run_id}] {node}: {frm} -> {next_status}"
                     f" ({duration:.1f}s) {detail}")

            transitions += 1
            if self.stop_after is not None and transitions >= self.stop_after:
                return state
