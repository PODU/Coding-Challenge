"""Route each demand along its shortest path and add it to each link's load.

Links over 100% utilization are reported, not treated as errors.
"""

from __future__ import annotations

from dataclasses import dataclass, field

from netmodel.demands_loader import Demand
from netmodel.model import Network
from netmodel.routing import Path, shortest_path


@dataclass(frozen=True)
class RouteResult:
    demand: Demand
    path: Path | None

    @property
    def routed(self) -> bool:
        return self.path is not None


@dataclass
class FlowResult:
    network: Network
    link_load: dict[str, float] = field(default_factory=dict)
    routes: list[RouteResult] = field(default_factory=list)
    unroutable: list[Demand] = field(default_factory=list)

    def utilization(self, link_id: str) -> float:
        """Load over capacity; 1.0 means 100%."""
        return self.link_load[link_id] / self.network.capacity_of(link_id)

    def utilization_pct(self, link_id: str) -> float:
        return self.utilization(link_id) * 100.0

    def over_capacity_links(self) -> list[str]:
        """Ids of links over 100%, sorted."""
        return sorted(
            link_id
            for link_id in self.link_load
            if self.utilization(link_id) > 1.0
        )

    def max_utilization(self, *, excluded_link: str | None = None) -> float:
        """Highest link utilization, or 0.0 if there are no links."""
        values = [
            self.utilization(link_id)
            for link_id in self.link_load
            if link_id != excluded_link
        ]
        return max(values, default=0.0)


def route_demands(
    network: Network,
    demands: list[Demand],
    excluded_link: str | None = None,
) -> FlowResult:
    """Route every demand. ``excluded_link`` is treated as failed and left out
    of the result."""
    link_load: dict[str, float] = {
        link_id: 0.0 for link_id in network.links if link_id != excluded_link
    }
    result = FlowResult(network=network, link_load=link_load)

    for demand in demands:
        path = shortest_path(network, demand.source, demand.destination, excluded_link)
        result.routes.append(RouteResult(demand=demand, path=path))
        if path is None:
            result.unroutable.append(demand)
            continue
        for link_id in path.links:
            link_load[link_id] += demand.amount

    return result
