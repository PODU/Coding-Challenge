"""Load the demands CSV.

Columns ``Source,Destination,Demand`` are required, in any order. Nodes that
aren't in the topology are allowed; those demands just end up unroutable.
"""

from __future__ import annotations

import csv
from dataclasses import dataclass
from pathlib import Path

from netmodel.model import InputError

_DEMAND_COLUMNS = ("Source", "Destination", "Demand")


@dataclass(frozen=True)
class Demand:
    source: str
    destination: str
    amount: float


def _require_columns(header: list[str] | None, expected: tuple[str, ...]) -> None:
    if header is None:
        raise InputError("file is empty (missing header row)")
    seen = [h.strip() for h in header]
    missing = [c for c in expected if c not in seen]
    if missing:
        raise InputError(
            f"missing required column(s): {', '.join(missing)}; "
            f"found header: {', '.join(seen)}"
        )


def load_demands(path: str | Path) -> list[Demand]:
    demands: list[Demand] = []
    try:
        handle = open(path, newline="", encoding="utf-8-sig")
    except OSError as exc:  # pragma: no cover - environment dependent
        raise InputError(f"cannot read demands file {path!r}: {exc}") from exc

    with handle:
        reader = csv.reader(handle)
        try:
            header = next(reader)
        except StopIteration:
            header = None
        _require_columns(header, _DEMAND_COLUMNS)
        assert header is not None
        index = {name.strip(): i for i, name in enumerate(header)}

        for row_number, raw in enumerate(reader, start=2):
            if not any(cell.strip() for cell in raw):
                continue
            if len(raw) < len(header):
                raise InputError(
                    f"expected {len(header)} columns, got {len(raw)}",
                    row=row_number,
                )

            def cell(name: str) -> str:
                return raw[index[name]].strip()

            source = cell("Source")
            destination = cell("Destination")
            if source == "":
                raise InputError("empty Source", row=row_number)
            if destination == "":
                raise InputError("empty Destination", row=row_number)

            amount_text = cell("Demand")
            if amount_text == "":
                raise InputError("empty Demand", row=row_number)
            try:
                amount = float(amount_text)
            except ValueError:
                raise InputError(f"non-numeric Demand: {amount_text!r}", row=row_number) from None
            if amount != amount:  # NaN
                raise InputError(f"non-numeric Demand: {amount_text!r}", row=row_number)
            if amount < 0:
                raise InputError(f"Demand must be >= 0, got {amount}", row=row_number)

            demands.append(Demand(source=source, destination=destination, amount=amount))

    return demands
