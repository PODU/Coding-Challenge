"""CLI smoke tests: exit codes and output through an injected fake service."""

import io
from datetime import UTC, datetime, time

import pytest

from scheduler.cli import EXIT_BAD_ARGS, EXIT_OK, EXIT_SLOT_TAKEN, main, parse_duration
from scheduler.domain.availability import AvailabilityRule, WeeklyAvailability
from scheduler.domain.clock import FixedClock
from scheduler.domain.interval import Interval
from scheduler.providers.fake_provider import FakeProvider
from scheduler.providers.ics import parse_events
from scheduler.service import SchedulingService

from .conftest import read_fixture


def utc(y, mo, d, h=0, m=0):
    return datetime(y, mo, d, h, m, tzinfo=UTC)


def make_service():
    window = Interval(utc(2026, 7, 2), utc(2026, 7, 3))
    events = parse_events(read_fixture("work.ics"), "work", window)
    events += parse_events(read_fixture("personal.ics"), "personal", window)
    availability = WeeklyAvailability(
        tz="UTC", rules=tuple(AvailabilityRule(d, time(9), time(17)) for d in range(5))
    )
    return SchedulingService(
        provider=FakeProvider(events),
        availability=availability,
        clock=FixedClock(utc(2026, 7, 1)),
        organizer_email="alice@example.com",
    )


class TestParseDuration:
    def test_minutes(self):
        assert parse_duration("30m").total_seconds() == 1800

    def test_hours_and_minutes(self):
        assert parse_duration("1h30m").total_seconds() == 5400

    def test_invalid(self):
        with pytest.raises(ValueError):
            parse_duration("abc")


class TestCommands:
    def test_events(self):
        out = io.StringIO()
        code = main(
            ["events", "--from", "2026-07-02", "--to", "2026-07-03"],
            service=make_service(),
            out=out,
        )
        assert code == EXIT_OK
        assert "2026-07-02T09:00" in out.getvalue()

    def test_slots(self):
        out = io.StringIO()
        code = main(
            ["slots", "--duration", "1h", "--from", "2026-07-02",
             "--to", "2026-07-03", "--step", "1h"],
            service=make_service(),
            out=out,
        )
        assert code == EXIT_OK
        lines = [ln for ln in out.getvalue().splitlines() if ln.strip()]
        assert len(lines) == 5  # 10,11,12,13,16

    def test_book_success(self):
        out = io.StringIO()
        code = main(
            ["book", "--start", "2026-07-02T11:00Z", "--duration", "1h",
             "--attendee", "Sam <sam@example.com>", "--title", "Intro"],
            service=make_service(),
            out=out,
        )
        assert code == EXIT_OK
        assert "booked:" in out.getvalue()

    def test_book_slot_taken(self):
        out = io.StringIO()
        code = main(
            ["book", "--start", "2026-07-02T09:00Z", "--duration", "1h",
             "--attendee", "Sam <sam@example.com>"],
            service=make_service(),
            out=out,
        )
        assert code == EXIT_SLOT_TAKEN

    def test_book_bad_email(self):
        out = io.StringIO()
        code = main(
            ["book", "--start", "2026-07-02T11:00Z", "--duration", "1h",
             "--attendee", "nope"],
            service=make_service(),
            out=out,
        )
        assert code == EXIT_BAD_ARGS

    def test_missing_required_arg(self):
        code = main(["slots", "--duration", "1h"], service=make_service(), out=io.StringIO())
        assert code == EXIT_BAD_ARGS
