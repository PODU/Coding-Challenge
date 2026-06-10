"""A small LangGraph-style runner, to avoid the dependency.

Nodes update a shared ``state`` dict; each edge is either a fixed next node or
a function of the state that picks one.
"""

from __future__ import annotations

from typing import Any, Callable

NodeFn = Callable[[dict], Any]
EdgeFn = Callable[[dict], str | None]

END = None


class Graph:
    def __init__(self) -> None:
        self._nodes: dict[str, NodeFn] = {}
        self._edges: dict[str, str | EdgeFn | None] = {}
        self.entry: str | None = None

    def node(self, name: str, fn: NodeFn) -> "Graph":
        self._nodes[name] = fn
        if self.entry is None:
            self.entry = name
        return self

    def edge(self, src: str, dst: str | None) -> "Graph":
        self._edges[src] = dst
        return self

    def conditional(self, src: str, chooser: EdgeFn) -> "Graph":
        self._edges[src] = chooser
        return self

    def set_entry(self, name: str) -> "Graph":
        self.entry = name
        return self

    def run(self, state: dict, *, max_steps: int = 1000) -> dict:
        current = self.entry
        steps = 0
        while current is not None:
            if steps >= max_steps:
                raise RuntimeError("graph exceeded max_steps (cycle?)")
            steps += 1
            self._nodes[current](state)
            nxt = self._edges.get(current, END)
            current = nxt(state) if callable(nxt) else nxt
        return state
