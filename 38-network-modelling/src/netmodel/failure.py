"""Single-link failure analysis.

Each link is removed in turn and all demands are re-routed. Scenarios are
ranked by most unroutable demands, then most over-capacity links, then highest
link utilization, with the link id as a final tie-break.
"""

from __future__ import annotations

from dataclasses import dataclass

from netmodel.demands_loader import Demand
from netmodel.flow import route_demands
from netmodel.model import Network


@dataclass(frozen=True)
class FailureScenario:
    failed_link_id: str
    unroutable_count: int
    over_capacity_count: int
    max_utilization: float  # 1.0 means 100%

    @property
    def max_utilization_pct(self) -> float:
        return self.max_utilization * 100.0

    def rank_key(self) -> tuple[int, int, float, str]:
        """Sorting ascending by this puts the worst scenario first."""
        return (
            -self.unroutable_count,
            -self.over_capacity_count,
            -self.max_utilization,
            self.failed_link_id,
        )


def analyze_link_failure(
    network: Network, demands: list[Demand], link_id: str
) -> FailureScenario:
    flow = route_demands(network, demands, excluded_link=link_id)
    return FailureScenario(
        failed_link_id=link_id,
        unroutable_count=len(flow.unroutable),
        over_capacity_count=len(flow.over_capacity_links()),
        max_utilization=flow.max_utilization(),
    )


def worst_case(network: Network, demands: list[Demand]) -> list[FailureScenario]:
    """Return a scenario for every link, worst first."""
    scenarios = (analyze_link_failure(network, demands, link_id) for link_id in network.links)
    return sorted(scenarios, key=FailureScenario.rank_key)
