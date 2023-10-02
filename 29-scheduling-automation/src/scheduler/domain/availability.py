"""Weekly availability rules and their expansion into UTC intervals.

Rules are wall-clock times in the organizer's timezone, so expanding them has
to deal with DST changes and windows that run past midnight.
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import UTC, datetime, time, timedelta
from zoneinfo import ZoneInfo

from .interval import Interval, merge


@dataclass(frozen=True)
class AvailabilityRule:
    """A bookable window on one weekday, in local time.

    ``weekday`` is 0 for Monday through 6 for Sunday, as in ``date.weekday()``.
    If ``end <= start`` the window runs past midnight into the next day
    (e.g. 22:00-02:00).
    """

    weekday: int
    start: time
    end: time

    def __post_init__(self) -> None:
        if not 0 <= self.weekday <= 6:
            raise ValueError(f"weekday must be in 0..6, got {self.weekday}")

    @property
    def crosses_midnight(self) -> bool:
        return self.end <= self.start


@dataclass(frozen=True)
class WeeklyAvailability:
    """Availability rules in the IANA timezone ``tz``."""

    tz: str
    rules: tuple[AvailabilityRule, ...]

    @property
    def zoneinfo(self) -> ZoneInfo:
        return ZoneInfo(self.tz)

    def expand(self, window: Interval) -> list[Interval]:
        """Return the bookable UTC intervals within *window*, merged and sorted."""
        tz = self.zoneinfo
        local_start = window.start.astimezone(tz)
        local_end = window.end.astimezone(tz)

        # Start a day early: a window that crosses midnight may begin the day
        # before *window* does.
        cursor = (local_start - timedelta(days=1)).date()
        last_date = local_end.date()

        produced: list[Interval] = []
        while cursor <= last_date:
            for rule in self.rules:
                if rule.weekday != cursor.weekday():
                    continue
                start_local = datetime.combine(cursor, rule.start, tzinfo=tz)
                end_date = cursor + (timedelta(days=1) if rule.crosses_midnight else timedelta())
                end_local = datetime.combine(end_date, rule.end, tzinfo=tz)
                candidate = Interval(
                    start_local.astimezone(UTC),
                    end_local.astimezone(UTC),
                )
                clipped = candidate.clip(window)
                if clipped is not None:
                    produced.append(clipped)
            cursor += timedelta(days=1)
        return merge(produced)
