"""Half-open UTC time intervals and set operations on them."""

from __future__ import annotations

from collections.abc import Iterable
from dataclasses import dataclass
from datetime import UTC, datetime, timedelta


def _ensure_utc_aware(value: datetime, label: str) -> None:
    if value.tzinfo is None or value.tzinfo.utcoffset(value) is None:
        raise ValueError(f"{label} must be timezone-aware, got naive {value!r}")


@dataclass(frozen=True, order=True)
class Interval:
    """A half-open ``[start, end)`` span of time.

    Both bounds must be timezone-aware and are stored in UTC; ``end`` must be
    after ``start``. Intervals sort by start, then end.
    """

    start: datetime
    end: datetime

    def __post_init__(self) -> None:
        _ensure_utc_aware(self.start, "start")
        _ensure_utc_aware(self.end, "end")
        # Store as UTC so equal instants in different zones compare equal.
        object.__setattr__(self, "start", self.start.astimezone(UTC))
        object.__setattr__(self, "end", self.end.astimezone(UTC))
        if self.end <= self.start:
            raise ValueError(f"end ({self.end}) must be after start ({self.start})")

    @property
    def duration(self) -> timedelta:
        return self.end - self.start

    def overlaps(self, other: Interval) -> bool:
        """Intervals that only touch (``a.end == b.start``) don't overlap."""
        return self.start < other.end and other.start < self.end

    def contains(self, moment: datetime) -> bool:
        _ensure_utc_aware(moment, "moment")
        moment = moment.astimezone(UTC)
        return self.start <= moment < self.end

    def clip(self, bound: Interval) -> Interval | None:
        """The overlap with *bound*, or None if there isn't one."""
        start = max(self.start, bound.start)
        end = min(self.end, bound.end)
        if end <= start:
            return None
        return Interval(start, end)


def merge(intervals: Iterable[Interval]) -> list[Interval]:
    """Sort and merge intervals, joining ones that overlap or touch."""
    ordered = sorted(intervals)
    if not ordered:
        return []
    merged: list[Interval] = [ordered[0]]
    for current in ordered[1:]:
        last = merged[-1]
        # <= so that touching intervals are joined too.
        if current.start <= last.end:
            if current.end > last.end:
                merged[-1] = Interval(last.start, current.end)
        else:
            merged.append(current)
    return merged


def subtract(base: Interval, busy: Iterable[Interval]) -> list[Interval]:
    """The parts of *base* not covered by *busy*, in order."""
    free: list[Interval] = [base]
    for block in merge(busy):
        clipped = block.clip(base)
        if clipped is None:
            continue
        next_free: list[Interval] = []
        for piece in free:
            if not piece.overlaps(clipped):
                next_free.append(piece)
                continue
            if piece.start < clipped.start:
                next_free.append(Interval(piece.start, clipped.start))
            if clipped.end < piece.end:
                next_free.append(Interval(clipped.end, piece.end))
        free = next_free
    return free


def intersect(a: Iterable[Interval], b: Iterable[Interval]) -> list[Interval]:
    """The times covered by both *a* and *b*, merged and sorted."""
    b_merged = merge(b)
    result: list[Interval] = []
    for ia in merge(a):
        for ib in b_merged:
            clipped = ia.clip(ib)
            if clipped is not None:
                result.append(clipped)
    return merge(result)
