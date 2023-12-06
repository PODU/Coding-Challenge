"""Tests for single-link-failure sweep and ranking."""

from __future__ import annotations

import pytest

from conftest import fixture
from netmodel.demands_loader import Demand
from netmodel.failure import worst_case
from netmodel.topology_loader import load_topology


def test_disconnecting_failure_ranks_first_by_unroutable():
    # Line A-B-C; demand A->C. Failing either link isolates an endpoint.
    net = load_topology(fixture("line.csv"))
    scenarios = worst_case(net, [Demand("A", "C", 10)])
    worst = scenarios[0]
    assert worst.unroutable_count == 1
    # Both L1 and L2 disconnect A<->C (tie on every metric) -> stable by link id.
    assert worst.failed_link_id == "L1"
    assert [s.failed_link_id for s in scenarios] == ["L1", "L2"]


def test_congestion_fallthrough_ranking():
    # No disconnection; ranking falls to over-capacity count, then max utilization.
    net = load_topology(fixture("congestion.csv"))
    scenarios = worst_case(net, [Demand("A", "B", 80)])
    order = [s.failed_link_id for s in scenarios]
    assert order == ["L1", "L2", "L3", "L4"]

    worst = scenarios[0]
    assert worst.failed_link_id == "L1"
    assert worst.unroutable_count == 0
    # Failing L1 reroutes 80 over L2 and L3 (cap 50 each) -> both > 100%.
    assert worst.over_capacity_count == 2
    assert worst.max_utilization == pytest.approx(1.6)

    # The dangling link L4 carries no rerouted load -> least damaging, sorts last.
    least = scenarios[-1]
    assert least.failed_link_id == "L4"
    assert least.over_capacity_count == 0
    assert least.max_utilization == pytest.approx(0.8)


def test_ranking_is_deterministic_across_runs():
    net = load_topology(fixture("congestion.csv"))
    demands = [Demand("A", "B", 80)]
    first = worst_case(net, demands)
    second = worst_case(net, demands)
    assert [s.failed_link_id for s in first] == [s.failed_link_id for s in second]
    assert [s.rank_key() for s in first] == [s.rank_key() for s in second]


def test_every_link_is_a_failure_target():
    net = load_topology(fixture("congestion.csv"))
    scenarios = worst_case(net, [Demand("A", "B", 80)])
    assert {s.failed_link_id for s in scenarios} == set(net.links)
