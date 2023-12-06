"""Command-line entry point (``python -m netmodel`` / ``netmodel``).

Subcommands:

* ``report``: route the demands and print link utilization and routes.
* ``path``: print the OSPF shortest path between two nodes.
* ``failure``: rank every single-link failure, worst first.

Exit codes: ``0`` success, ``1`` bad CLI usage, ``2`` malformed/invalid input.
"""

from __future__ import annotations

import argparse
import sys
from typing import Sequence, TextIO

from netmodel import report
from netmodel.demands_loader import load_demands
from netmodel.failure import worst_case
from netmodel.flow import route_demands
from netmodel.model import InputError
from netmodel.routing import shortest_path
from netmodel.topology_loader import load_topology

EXIT_OK = 0
EXIT_USAGE = 1
EXIT_INPUT = 2


class _CliError(Exception):
    """Bad command-line usage (exit code 1)."""


class _ArgumentParser(argparse.ArgumentParser):
    """Raises on usage errors instead of exiting, so we pick the exit code."""

    def error(self, message: str) -> "None":  # type: ignore[override]
        raise _CliError(message)


def build_parser() -> argparse.ArgumentParser:
    parser = _ArgumentParser(
        prog="netmodel",
        description="Network modelling tool: OSPF routing, demand flow and failure analysis.",
    )
    sub = parser.add_subparsers(dest="command", required=True)

    report_cmd = sub.add_parser(
        "report", help="route demands and emit link-utilization + route reports"
    )
    report_cmd.add_argument("--links", required=True, help="links CSV path")
    report_cmd.add_argument("--demands", required=True, help="demands CSV path")
    report_cmd.add_argument("--out", help="write the link-utilization CSV to this file")

    path_cmd = sub.add_parser("path", help="print the OSPF shortest path between two nodes")
    path_cmd.add_argument("--links", required=True, help="links CSV path")
    path_cmd.add_argument("--src", required=True, help="source node")
    path_cmd.add_argument("--dst", required=True, help="destination node")

    failure_cmd = sub.add_parser(
        "failure", help="emit the ranked single-link-failure report"
    )
    failure_cmd.add_argument("--links", required=True, help="links CSV path")
    failure_cmd.add_argument("--demands", required=True, help="demands CSV path")
    failure_cmd.add_argument("--out", help="write the failure CSV to this file")

    return parser


def _cmd_report(args: argparse.Namespace, out: TextIO) -> int:
    network = load_topology(args.links)
    demands = load_demands(args.demands)
    flow = route_demands(network, demands)

    link_csv = report.link_utilization_report(flow)
    if args.out:
        with open(args.out, "w", newline="", encoding="utf-8") as handle:
            handle.write(link_csv)

    out.write("# Link Utilization\n")
    out.write(link_csv)
    out.write("\n# Routes\n")
    out.write(report.route_report(flow))
    return EXIT_OK


def _cmd_path(args: argparse.Namespace, out: TextIO) -> int:
    network = load_topology(args.links)
    path = shortest_path(network, args.src, args.dst)
    if path is None:
        out.write(f"No path from {args.src} to {args.dst}\n")
    else:
        out.write(f"{path.as_str()} weight={report._fmt_number(path.weight)}\n")
    return EXIT_OK


def _cmd_failure(args: argparse.Namespace, out: TextIO, err: TextIO) -> int:
    network = load_topology(args.links)
    demands = load_demands(args.demands)
    scenarios = worst_case(network, demands)

    failure_csv = report.failure_report(scenarios)
    if args.out:
        with open(args.out, "w", newline="", encoding="utf-8") as handle:
            handle.write(failure_csv)

    out.write(failure_csv)
    if scenarios:
        worst = scenarios[0]
        err.write(
            "Worst-case single-link failure: "
            f"{worst.failed_link_id} "
            f"(unroutable={worst.unroutable_count}, "
            f"over_capacity_links={worst.over_capacity_count}, "
            f"max_utilization={worst.max_utilization_pct:.2f}%)\n"
        )
    else:
        err.write("No links to fail.\n")
    return EXIT_OK


def main(
    argv: Sequence[str] | None = None,
    *,
    out: TextIO | None = None,
    err: TextIO | None = None,
) -> int:
    """Run the CLI and return the exit code."""
    out = out if out is not None else sys.stdout
    err = err if err is not None else sys.stderr

    try:
        args = build_parser().parse_args(argv)
    except _CliError as exc:
        err.write(f"usage error: {exc}\n")
        return EXIT_USAGE

    try:
        if args.command == "report":
            return _cmd_report(args, out)
        if args.command == "path":
            return _cmd_path(args, out)
        if args.command == "failure":
            return _cmd_failure(args, out, err)
        err.write(f"unknown command: {args.command}\n")  # pragma: no cover
        return EXIT_USAGE
    except InputError as exc:
        err.write(f"input error: {exc}\n")
        return EXIT_INPUT


def main_cli() -> None:
    sys.exit(main())


if __name__ == "__main__":  # pragma: no cover
    main_cli()
