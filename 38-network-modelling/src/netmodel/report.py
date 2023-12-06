"""CSV reports. Numbers are only rounded here, and rows always come out in
the same order so output can be diffed between runs."""

from __future__ import annotations

import csv
import io

from netmodel.failure import FailureScenario
from netmodel.flow import FlowResult

_PCT_DECIMALS = 2


def _fmt_number(value: float) -> str:
    """Print whole numbers without a trailing ``.0``."""
    if value == int(value):
        return str(int(value))
    return repr(value)


def _fmt_pct(value: float) -> str:
    return f"{value:.{_PCT_DECIMALS}f}"


def _rows_to_csv(header: list[str], rows: list[list[str]]) -> str:
    buffer = io.StringIO()
    writer = csv.writer(buffer, lineterminator="\n")
    writer.writerow(header)
    writer.writerows(rows)
    return buffer.getvalue()


def link_utilization_rows(flow: FlowResult) -> list[list[str]]:
    """Per-link load/utilization rows, sorted by ``LinkId``."""
    rows: list[list[str]] = []
    for link_id in sorted(flow.link_load):
        link = flow.network.links[link_id]
        pct = flow.utilization_pct(link_id)
        rows.append(
            [
                link_id,
                link.start,
                link.end,
                _fmt_number(link.capacity),
                _fmt_number(flow.link_load[link_id]),
                _fmt_pct(pct),
                "true" if pct > 100.0 else "false",
            ]
        )
    return rows


def link_utilization_report(flow: FlowResult) -> str:
    """CSV: ``LinkId,Start,End,Capacity,Load,UtilizationPct,OverCapacity``."""
    header = ["LinkId", "Start", "End", "Capacity", "Load", "UtilizationPct", "OverCapacity"]
    return _rows_to_csv(header, link_utilization_rows(flow))


def route_rows(flow: FlowResult) -> list[list[str]]:
    """One row per demand in input order."""
    rows: list[list[str]] = []
    for route in flow.routes:
        demand = route.demand
        rows.append(
            [
                demand.source,
                demand.destination,
                _fmt_number(demand.amount),
                route.path.as_str() if route.path is not None else "",
                "yes" if route.routed else "no",
            ]
        )
    return rows


def route_report(flow: FlowResult) -> str:
    """CSV: ``Source,Destination,Demand,Path,Routed``."""
    header = ["Source", "Destination", "Demand", "Path", "Routed"]
    return _rows_to_csv(header, route_rows(flow))


def failure_rows(scenarios: list[FailureScenario]) -> list[list[str]]:
    """One row per failure scenario, in the given (worst-first) order."""
    return [
        [
            scenario.failed_link_id,
            str(scenario.unroutable_count),
            str(scenario.over_capacity_count),
            _fmt_pct(scenario.max_utilization_pct),
        ]
        for scenario in scenarios
    ]


def failure_report(scenarios: list[FailureScenario]) -> str:
    """CSV: ``FailedLinkId,UnroutableDemands,OverCapacityLinks,MaxUtilizationPct``."""
    header = ["FailedLinkId", "UnroutableDemands", "OverCapacityLinks", "MaxUtilizationPct"]
    return _rows_to_csv(header, failure_rows(scenarios))
