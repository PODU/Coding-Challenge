"""Free-slot computation. Takes "now" as an argument and does no I/O."""

from __future__ import annotations

from collections.abc import Iterable
from dataclasses import dataclass, field
from datetime import datetime, timedelta

from .availability import WeeklyAvailability
from .interval import Interval, merge, subtract


@dataclass(frozen=True)
class SlotRequest:
    """What kind of slot to look for.

    Candidate slots start every ``step``, so a step shorter than ``duration``
    gives overlapping slots. The buffers keep time free around busy events, and
    slots starting before ``now + min_notice`` are dropped.
    """

    duration: timedelta
    window: Interval
    step: timedelta = field(default_factory=lambda: timedelta(minutes=15))
    buffer_before: timedelta = field(default_factory=lambda: timedelta(0))
    buffer_after: timedelta = field(default_factory=lambda: timedelta(0))
    min_notice: timedelta = field(default_factory=lambda: timedelta(0))

    def __post_init__(self) -> None:
        if self.duration <= timedelta(0):
            raise ValueError(f"duration must be positive, got {self.duration}")
        if self.step <= timedelta(0):
            raise ValueError(f"step must be positive, got {self.step}")
        if self.buffer_before < timedelta(0):
            raise ValueError("buffer_before must not be negative")
        if self.buffer_after < timedelta(0):
            raise ValueError("buffer_after must not be negative")
        if self.min_notice < timedelta(0):
            raise ValueError("min_notice must not be negative")


def _apply_buffers(
    busy: Iterable[Interval],
    buffer_before: timedelta,
    buffer_after: timedelta,
) -> list[Interval]:
    return merge(Interval(b.start - buffer_before, b.end + buffer_after) for b in busy)


def _slice_into_slots(
    free: Interval, duration: timedelta, step: timedelta
) -> list[Interval]:
    slots: list[Interval] = []
    cursor = free.start
    while cursor + duration <= free.end:
        slots.append(Interval(cursor, cursor + duration))
        cursor += step
    return slots


def free_slots(
    availability: WeeklyAvailability,
    busy: Iterable[Interval],
    req: SlotRequest,
    now: datetime,
) -> list[Interval]:
    """Return the bookable slots for *req*, in order.

    Availability minus the (buffered) busy times gives the free time; anything
    before the notice cutoff is trimmed off, and the rest is cut into slots.
    """
    if now.tzinfo is None:
        raise ValueError("now must be timezone-aware")

    busy_mask = _apply_buffers(busy, req.buffer_before, req.buffer_after)
    notice_floor = max(now + req.min_notice, req.window.start)

    slots: list[Interval] = []
    for window in availability.expand(req.window):
        for piece in subtract(window, busy_mask):
            if piece.end <= notice_floor:
                continue
            if piece.start < notice_floor:
                piece = Interval(notice_floor, piece.end)
            slots.extend(_slice_into_slots(piece, req.duration, req.step))
    slots.sort()
    return slots
