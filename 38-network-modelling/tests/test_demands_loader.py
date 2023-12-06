"""Tests for demands loading and validation."""

from __future__ import annotations

import pytest

from netmodel.demands_loader import Demand, load_demands
from netmodel.model import InputError


def _write(tmp_path, text: str):
    path = tmp_path / "demands.csv"
    path.write_text(text, encoding="utf-8")
    return str(path)


def test_loads_demands(tmp_path):
    path = _write(
        tmp_path,
        "Source,Destination,Demand\nA,D,60\nB,E,20\n",
    )
    demands = load_demands(path)
    assert demands == [Demand("A", "D", 60.0), Demand("B", "E", 20.0)]


def test_blank_lines_and_whitespace_handled(tmp_path):
    path = _write(
        tmp_path,
        "Source,Destination,Demand\n\n A , D , 60 \n\n",
    )
    demands = load_demands(path)
    assert demands == [Demand("A", "D", 60.0)]


def test_negative_demand_rejected(tmp_path):
    path = _write(tmp_path, "Source,Destination,Demand\nA,D,-1\n")
    with pytest.raises(InputError) as exc:
        load_demands(path)
    assert "row 2" in str(exc.value)


def test_zero_demand_allowed(tmp_path):
    path = _write(tmp_path, "Source,Destination,Demand\nA,D,0\n")
    assert load_demands(path) == [Demand("A", "D", 0.0)]


def test_non_numeric_demand_rejected(tmp_path):
    path = _write(tmp_path, "Source,Destination,Demand\nA,D,lots\n")
    with pytest.raises(InputError) as exc:
        load_demands(path)
    assert "non-numeric" in str(exc.value).lower()


def test_missing_column_rejected(tmp_path):
    path = _write(tmp_path, "Source,Destination\nA,D\n")
    with pytest.raises(InputError) as exc:
        load_demands(path)
    assert "Demand" in str(exc.value)


def test_empty_demands_file_returns_empty_list(tmp_path):
    path = _write(tmp_path, "Source,Destination,Demand\n")
    assert load_demands(path) == []
