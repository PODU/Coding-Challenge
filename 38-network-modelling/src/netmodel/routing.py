"""OSPF shortest paths (lowest total weight) using Dijkstra.

Traffic is not split across equal-cost paths. Instead the path whose node
names sort first wins, which falls out of ordering the heap on
``(cost, node_path, link_path)``.
"""

from __future__ import annotations

import heapq
from dataclasses import dataclass

from netmodel.model import Network


@dataclass(frozen=True)
class Path:
    nodes: tuple[str, ...]
    links: tuple[str, ...]
    weight: float

    def as_str(self) -> str:
        """E.g. ``A->B->D``."""
        return "->".join(self.nodes)


def shortest_path(
    network: Network,
    src: str,
    dst: str,
    excluded_link: str | None = None,
) -> Path | None:
    """Return the shortest path, or ``None`` if there isn't one. A path from
    a node to itself has no links and zero weight."""
    if src == dst:
        return Path(nodes=(src,), links=(), weight=0.0)
    if src not in network.nodes or dst not in network.nodes:
        return None

    # Comparing whole tuples gives the tie-break described at the top.
    heap: list[tuple[float, tuple[str, ...], tuple[str, ...]]] = [(0.0, (src,), ())]
    finalized: set[str] = set()

    while heap:
        cost, node_path, link_path = heapq.heappop(heap)
        node = node_path[-1]
        if node in finalized:
            continue
        finalized.add(node)
        if node == dst:
            return Path(nodes=node_path, links=link_path, weight=cost)

        for neighbor, link_id, weight in network.neighbors(node):
            if link_id == excluded_link or neighbor in finalized:
                continue
            heapq.heappush(
                heap,
                (cost + weight, node_path + (neighbor,), link_path + (link_id,)),
            )

    return None
