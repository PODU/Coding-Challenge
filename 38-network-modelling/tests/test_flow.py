"""Tests for demand routing, link load and utilization math."""

from __future__ import annotations

import pytest

from conftest import fixture
from netmodel.demands_loader import Demand
from netmodel.flow import route_demands
from netmodel.topology_loader import load_topology


def test_single_demand_loads_only_its_path():
    net = load_topology(fixture("diamond.csv"))
    flow = route_demands(net, [Demand("A", "D", 10)])
    assert flow.link_load["L1"] == 10  # A-B
    assert flow.link_load["L2"] == 10  # B-D
    assert flow.link_load["L3"] == 0
    assert flow.link_load["L4"] == 0
    assert flow.link_load["L5"] == 0
    assert flow.unroutable == []


def test_demands_sharing_a_link_sum():
    net = load_topology(fixture("diamond.csv"))
    flow = route_demands(net, [Demand("A", "D", 10), Demand("A", "B", 5)])
    # Both demands traverse L1 (A-B): 10 + 5 = 15.
    assert flow.link_load["L1"] == 15
    assert flow.link_load["L2"] == 10


def test_utilization_math_and_over_capacity():
    net = load_topology(fixture("diamond.csv"))
    # Capacity is 100; route 150 over A->B->D.
    flow = route_demands(net, [Demand("A", "D", 150)])
    assert flow.utilization("L1") == pytest.approx(1.5)
    assert flow.utilization_pct("L1") == pytest.approx(150.0)
    assert "L1" in flow.over_capacity_links()
    assert "L2" in flow.over_capacity_links()
    assert flow.max_utilization() == pytest.approx(1.5)


def test_exactly_100_percent_is_not_over_capacity():
    net = load_topology(fixture("diamond.csv"))
    flow = route_demands(net, [Demand("A", "D", 100)])
    assert flow.utilization_pct("L1") == pytest.approx(100.0)
    # OverCapacity is strictly > 100%.
    assert flow.over_capacity_links() == []


def test_unroutable_demand_recorded_no_load():
    net = load_topology(fixture("disconnected.csv"))
    flow = route_demands(net, [Demand("A", "C", 10)])
    assert flow.unroutable == [Demand("A", "C", 10)]
    assert all(load == 0 for load in flow.link_load.values())


def test_unknown_node_demand_unroutable():
    net = load_topology(fixture("diamond.csv"))
    flow = route_demands(net, [Demand("A", "Z", 10)])
    assert len(flow.unroutable) == 1
    assert all(load == 0 for load in flow.link_load.values())


def test_source_equals_destination_routes_zero_load():
    net = load_topology(fixture("diamond.csv"))
    flow = route_demands(net, [Demand("A", "A", 10)])
    assert flow.unroutable == []
    assert all(load == 0 for load in flow.link_load.values())
    assert flow.routes[0].routed is True


def test_empty_demands_all_links_zero():
    net = load_topology(fixture("diamond.csv"))
    flow = route_demands(net, [])
    assert set(flow.link_load) == {"L1", "L2", "L3", "L4", "L5"}
    assert all(load == 0 for load in flow.link_load.values())
    assert flow.unroutable == []
