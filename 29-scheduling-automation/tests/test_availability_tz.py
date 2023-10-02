"""Timezone, DST and midnight-crossing tests for availability expansion."""

from datetime import UTC, datetime, time

from scheduler.domain.availability import AvailabilityRule, WeeklyAvailability
from scheduler.domain.interval import Interval


def utc(y, mo, d, h=0, m=0):
    return datetime(y, mo, d, h, m, tzinfo=UTC)


class TestNonUtcTimezone:
    def test_bst_window_maps_to_utc(self):
        # 2026-07-01 is a Wednesday; London is BST (UTC+1) in July.
        avail = WeeklyAvailability(
            tz="Europe/London",
            rules=(AvailabilityRule(weekday=2, start=time(9), end=time(17)),),
        )
        window = Interval(utc(2026, 7, 1), utc(2026, 7, 2))
        intervals = avail.expand(window)
        assert intervals == [Interval(utc(2026, 7, 1, 8), utc(2026, 7, 1, 16))]


class TestDST:
    def test_spring_forward_window_loses_an_hour_in_utc(self):
        # 2026-03-29 (Sunday) London clocks jump 01:00 -> 02:00.
        # Local 00:00-06:00 spans the gap: 6 wall hours but 5 UTC hours.
        avail = WeeklyAvailability(
            tz="Europe/London",
            rules=(AvailabilityRule(weekday=6, start=time(0), end=time(6)),),
        )
        window = Interval(utc(2026, 3, 29), utc(2026, 3, 30))
        intervals = avail.expand(window)
        assert len(intervals) == 1
        assert intervals[0].start == utc(2026, 3, 29, 0)
        assert intervals[0].end == utc(2026, 3, 29, 5)


class TestMidnightCrossing:
    def test_window_crossing_midnight_spans_two_days(self):
        # Wednesday 22:00 -> 02:00 (next day), in UTC.
        avail = WeeklyAvailability(
            tz="UTC",
            rules=(AvailabilityRule(weekday=2, start=time(22), end=time(2)),),
        )
        window = Interval(utc(2026, 7, 1), utc(2026, 7, 3))
        intervals = avail.expand(window)
        assert intervals == [Interval(utc(2026, 7, 1, 22), utc(2026, 7, 2, 2))]

    def test_crossing_window_clipped_at_window_end(self):
        avail = WeeklyAvailability(
            tz="UTC",
            rules=(AvailabilityRule(weekday=2, start=time(22), end=time(2)),),
        )
        # Window ends at 00:30, so the crossing interval is clipped there.
        window = Interval(utc(2026, 7, 1, 23), utc(2026, 7, 2, 0, 30))
        intervals = avail.expand(window)
        assert intervals == [Interval(utc(2026, 7, 1, 23), utc(2026, 7, 2, 0, 30))]
