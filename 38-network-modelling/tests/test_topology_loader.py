"""Tests for topology loading and validation."""

from __future__ import annotations

import pytest

from conftest import fixture
from netmodel.model import InputError
from netmodel.topology_loader import load_topology


def _write(tmp_path, text: str):
    path = tmp_path / "links.csv"
    path.write_text(text, encoding="utf-8")
    return str(path)


def test_loads_known_topology():
    net = load_topology(fixture("diamond.csv"))
    assert set(net.nodes) == {"A", "B", "C", "D"}
    assert set(net.links) == {"L1", "L2", "L3", "L4", "L5"}
    # Adjacency is bidirectional: each undirected link appears on both endpoints.
    assert ("B", "L1", 1.0) in net.neighbors("A")
    assert ("A", "L1", 1.0) in net.neighbors("B")
    assert net.link_endpoints("L1") == ("A", "B")
    assert net.capacity_of("L1") == 100.0


def test_duplicate_link_id_rejected(tmp_path):
    path = _write(
        tmp_path,
        "LinkId,Start,End,Capacity,Weight\nL1,A,B,100,1\nL1,A,C,100,1\n",
    )
    with pytest.raises(InputError) as exc:
        load_topology(path)
    assert "duplicate" in str(exc.value).lower()
    assert "row 3" in str(exc.value)


def test_self_loop_rejected(tmp_path):
    path = _write(tmp_path, "LinkId,Start,End,Capacity,Weight\nL1,A,A,100,1\n")
    with pytest.raises(InputError) as exc:
        load_topology(path)
    assert "self-loop" in str(exc.value).lower()
    assert "row 2" in str(exc.value)


@pytest.mark.parametrize("capacity", ["0", "-5"])
def test_non_positive_capacity_rejected(tmp_path, capacity):
    path = _write(tmp_path, f"LinkId,Start,End,Capacity,Weight\nL1,A,B,{capacity},1\n")
    with pytest.raises(InputError) as exc:
        load_topology(path)
    assert "Capacity" in str(exc.value)
    assert "row 2" in str(exc.value)


def test_negative_weight_rejected(tmp_path):
    path = _write(tmp_path, "LinkId,Start,End,Capacity,Weight\nL1,A,B,100,-1\n")
    with pytest.raises(InputError) as exc:
        load_topology(path)
    assert "Weight" in str(exc.value)


def test_non_numeric_field_rejected(tmp_path):
    path = _write(tmp_path, "LinkId,Start,End,Capacity,Weight\nL1,A,B,abc,1\n")
    with pytest.raises(InputError) as exc:
        load_topology(path)
    assert "non-numeric" in str(exc.value).lower()
    assert "row 2" in str(exc.value)


def test_missing_column_rejected(tmp_path):
    path = _write(tmp_path, "LinkId,Start,End,Capacity\nL1,A,B,100\n")
    with pytest.raises(InputError) as exc:
        load_topology(path)
    assert "Weight" in str(exc.value)


def test_empty_file_rejected(tmp_path):
    path = _write(tmp_path, "")
    with pytest.raises(InputError):
        load_topology(path)


def test_blank_lines_and_bom_handled(tmp_path):
    text = "﻿LinkId,Start,End,Capacity,Weight\n\nL1, A , B ,100,1\n\n"
    path = _write(tmp_path, text)
    net = load_topology(path)
    assert set(net.links) == {"L1"}
    # Whitespace around node names is trimmed.
    assert net.link_endpoints("L1") == ("A", "B")


def test_parallel_links_kept(tmp_path):
    path = _write(
        tmp_path,
        "LinkId,Start,End,Capacity,Weight\nL1,A,B,100,1\nL2,A,B,50,3\n",
    )
    net = load_topology(path)
    assert set(net.links) == {"L1", "L2"}
    assert len(net.neighbors("A")) == 2
