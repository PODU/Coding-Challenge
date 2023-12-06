"""Node, Link and Network.

Links are undirected with the same capacity both ways. Each is stored once,
and ``Network.adjacency`` lists it under both of its ends.
"""

from __future__ import annotations

from dataclasses import dataclass, field


class InputError(ValueError):
    """Bad input data. ``row`` is the 1-based CSV row, if known."""

    def __init__(self, message: str, *, row: int | None = None) -> None:
        self.row = row
        if row is not None:
            message = f"row {row}: {message}"
        super().__init__(message)


@dataclass(frozen=True)
class Node:
    """Coordinates are unused for now."""

    name: str
    x: float | None = None
    y: float | None = None


@dataclass(frozen=True)
class Link:
    id: str
    start: str
    end: str
    capacity: float
    weight: float


# (neighbor_name, link_id, weight)
AdjEntry = tuple[str, str, float]


@dataclass
class Network:
    nodes: dict[str, Node] = field(default_factory=dict)
    links: dict[str, Link] = field(default_factory=dict)
    adjacency: dict[str, list[AdjEntry]] = field(default_factory=dict)

    def add_node(self, name: str) -> Node:
        """Return node ``name``, creating it if needed."""
        node = self.nodes.get(name)
        if node is None:
            node = Node(name)
            self.nodes[name] = node
            self.adjacency.setdefault(name, [])
        return node

    def add_link(self, link: Link) -> None:
        """Raises :class:`InputError` on a duplicate id or a self-loop."""
        if link.id in self.links:
            raise InputError(f"duplicate LinkId {link.id!r}")
        if link.start == link.end:
            raise InputError(f"self-loop link {link.id!r} on node {link.start!r}")
        self.add_node(link.start)
        self.add_node(link.end)
        self.links[link.id] = link
        self.adjacency[link.start].append((link.end, link.id, link.weight))
        self.adjacency[link.end].append((link.start, link.id, link.weight))

    def neighbors(self, node: str) -> list[AdjEntry]:
        return self.adjacency.get(node, [])

    def link_endpoints(self, link_id: str) -> tuple[str, str]:
        link = self.links[link_id]
        return (link.start, link.end)

    def capacity_of(self, link_id: str) -> float:
        return self.links[link_id].capacity
