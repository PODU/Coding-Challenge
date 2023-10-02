"""Recurrence (RRULE/EXDATE) and all-day expansion tests against fixtures."""

from datetime import UTC, datetime

from scheduler.domain.interval import Interval
from scheduler.providers.ics import parse_events

from .conftest import read_fixture


def utc(y, mo, d, h=0, m=0):
    return datetime(y, mo, d, h, m, tzinfo=UTC)


class TestRRule:
    def test_weekly_with_exdate(self):
        # Mondays from 2026-07-06, COUNT=4 -> Jul 6,13,20,27; EXDATE removes Jul 20.
        window = Interval(utc(2026, 7, 1), utc(2026, 8, 1))
        events = parse_events(read_fixture("recurring.ics"), "work", window)
        days = sorted(e.start.day for e in events)
        assert days == [6, 13, 27]
        assert all(e.start.hour == 10 for e in events)

    def test_window_clips_occurrences(self):
        # Only the first two Mondays fall inside this narrower window.
        window = Interval(utc(2026, 7, 1), utc(2026, 7, 15))
        events = parse_events(read_fixture("recurring.ics"), "work", window)
        assert sorted(e.start.day for e in events) == [6, 13]


class TestAllDay:
    def test_all_day_anchored_to_organizer_tz(self):
        # All-day 2026-07-03 in London (BST) = 2026-07-02 23:00Z .. 2026-07-03 23:00Z.
        window = Interval(utc(2026, 7, 1), utc(2026, 7, 6))
        events = parse_events(
            read_fixture("all_day.ics"), "personal", window, organizer_tz="Europe/London"
        )
        assert len(events) == 1
        ev = events[0]
        assert ev.event_type == "all_day"
        assert ev.start == utc(2026, 7, 2, 23)
        assert ev.end == utc(2026, 7, 3, 23)

    def test_all_day_in_utc(self):
        window = Interval(utc(2026, 7, 1), utc(2026, 7, 6))
        events = parse_events(read_fixture("all_day.ics"), "personal", window, "UTC")
        assert events[0].start == utc(2026, 7, 3)
        assert events[0].end == utc(2026, 7, 4)
