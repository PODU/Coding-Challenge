"""netmodel: model routing, load and link failures in an IP network.

The main pieces are importable from the package::

    from netmodel import Network, load_topology, route_demands, worst_case
"""

from __future__ import annotations

from netmodel.demands_loader import Demand, load_demands
from netmodel.failure import FailureScenario, worst_case
from netmodel.flow import FlowResult, RouteResult, route_demands
from netmodel.model import InputError, Link, Network, Node
from netmodel.routing import Path, shortest_path
from netmodel.topology_loader import load_topology

__version__ = "1.0.0"

__all__ = [
    "__version__",
    "Demand",
    "load_demands",
    "FailureScenario",
    "worst_case",
    "FlowResult",
    "RouteResult",
    "route_demands",
    "InputError",
    "Link",
    "Network",
    "Node",
    "Path",
    "shortest_path",
    "load_topology",
]
