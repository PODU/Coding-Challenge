"""Tests for OSPF / Dijkstra shortest-path routing."""

from __future__ import annotations

from conftest import fixture
from netmodel.routing import shortest_path
from netmodel.topology_loader import load_topology


def test_lowest_weight_not_fewest_hops():
    net = load_topology(fixture("diamond.csv"))
    path = shortest_path(net, "A", "D")
    assert path is not None
    # A->B->D (weight 2) beats the 1-hop A->D (weight 10) and A->C->D (weight 5).
    assert path.nodes == ("A", "B", "D")
    assert path.links == ("L1", "L2")
    assert path.weight == 2.0


def test_equal_cost_lexicographic_tie_break():
    net = load_topology(fixture("equal_cost.csv"))
    path = shortest_path(net, "A", "D")
    assert path is not None
    assert path.weight == 2.0
    # A->B->D and A->C->D both cost 2; lexicographic node order picks B over C.
    assert path.nodes == ("A", "B", "D")


def test_unreachable_returns_none():
    net = load_topology(fixture("disconnected.csv"))
    assert shortest_path(net, "A", "C") is None


def test_unknown_node_returns_none():
    net = load_topology(fixture("diamond.csv"))
    assert shortest_path(net, "A", "Z") is None
    assert shortest_path(net, "Z", "A") is None


def test_excluded_link_reroutes():
    net = load_topology(fixture("diamond.csv"))
    # Exclude B->D: best remaining route is A->C->D (weight 5).
    path = shortest_path(net, "A", "D", excluded_link="L2")
    assert path is not None
    assert path.nodes == ("A", "C", "D")
    assert path.links == ("L3", "L4")
    assert path.weight == 5.0


def test_zero_weight_link_handled():
    net = load_topology(fixture("zero_weight.csv"))
    path = shortest_path(net, "A", "D")
    assert path is not None
    # A->B (weight 0) then B->D (weight 1) = 1, beats direct A->D (weight 5).
    assert path.nodes == ("A", "B", "D")
    assert path.weight == 1.0


def test_source_equals_destination_is_trivial():
    net = load_topology(fixture("diamond.csv"))
    path = shortest_path(net, "A", "A")
    assert path is not None
    assert path.nodes == ("A",)
    assert path.links == ()
    assert path.weight == 0.0


def test_parallel_links_pick_lower_weight(tmp_path):
    path_csv = tmp_path / "links.csv"
    path_csv.write_text(
        "LinkId,Start,End,Capacity,Weight\nL1,A,B,100,5\nL2,A,B,100,2\n",
        encoding="utf-8",
    )
    net = load_topology(str(path_csv))
    path = shortest_path(net, "A", "B")
    assert path is not None
    assert path.weight == 2.0
    assert path.links == ("L2",)


def test_determinism_repeated_runs():
    net = load_topology(fixture("equal_cost.csv"))
    first = shortest_path(net, "A", "D")
    second = shortest_path(net, "A", "D")
    assert first == second
