"""Load the links CSV into a :class:`~netmodel.model.Network`.

Columns ``LinkId,Start,End,Capacity,Weight`` are required, in any order. The
first bad row stops loading with an error naming the row.
"""

from __future__ import annotations

import csv
from pathlib import Path

from netmodel.model import InputError, Link, Network

_LINK_COLUMNS = ("LinkId", "Start", "End", "Capacity", "Weight")


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


def _parse_number(value: str, field: str, row: int, *, kind: str) -> float:
    text = value.strip()
    if text == "":
        raise InputError(f"empty {field}", row=row)
    try:
        number = float(text)
    except ValueError:
        raise InputError(f"non-numeric {field}: {value!r}", row=row) from None
    if number != number:  # NaN
        raise InputError(f"non-numeric {field}: {value!r}", row=row)
    if kind == "capacity" and number <= 0:
        raise InputError(f"Capacity must be > 0, got {number}", row=row)
    if kind == "weight" and number < 0:
        raise InputError(f"Weight must be >= 0, got {number}", row=row)
    return number


def load_topology(path: str | Path) -> Network:
    network = Network()
    try:
        handle = open(path, newline="", encoding="utf-8-sig")
    except OSError as exc:  # pragma: no cover - environment dependent
        raise InputError(f"cannot read links file {path!r}: {exc}") from exc

    with handle:
        reader = csv.reader(handle)
        try:
            header = next(reader)
        except StopIteration:
            header = None
        _require_columns(header, _LINK_COLUMNS)
        assert header is not None
        index = {name.strip(): i for i, name in enumerate(header)}

        # Row numbers count the header as row 1.
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

            link_id = cell("LinkId")
            if link_id == "":
                raise InputError("empty LinkId", row=row_number)
            start = cell("Start")
            end = cell("End")
            if start == "":
                raise InputError("empty Start node", row=row_number)
            if end == "":
                raise InputError("empty End node", row=row_number)
            capacity = _parse_number(cell("Capacity"), "Capacity", row_number, kind="capacity")
            weight = _parse_number(cell("Weight"), "Weight", row_number, kind="weight")

            link = Link(id=link_id, start=start, end=end, capacity=capacity, weight=weight)
            try:
                network.add_link(link)
            except InputError as exc:
                raise InputError(str(exc), row=row_number) from None

    return network
