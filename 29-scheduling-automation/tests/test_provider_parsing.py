"""ICS parsing, filtering and multi-calendar consolidation tests."""

from datetime import UTC, datetime

from scheduler.domain.interval import Interval
from scheduler.providers.fake_provider import FakeProvider
from scheduler.providers.ics import parse_events

from .conftest import read_fixture


def utc(y, mo, d, h=0, m=0):
    return datetime(y, mo, d, h, m, tzinfo=UTC)


WINDOW = Interval(utc(2026, 7, 2), utc(2026, 7, 3))


class TestParsingFilters:
    def test_cancelled_excluded_transparent_marked(self):
        events = parse_events(read_fixture("work.ics"), "work", WINDOW)
        uids = {e.uid for e in events}
        assert "cancelled-1@work" not in uids  # STATUS:CANCELLED dropped
        transparent = next(e for e in events if e.uid == "transparent-1@work")
        assert transparent.transparent is True
        assert transparent.event_type == "free"
        assert not transparent.blocks()

    def test_blocking_events_identified(self):
        events = parse_events(read_fixture("work.ics"), "work", WINDOW)
        blocking = [e.uid for e in events if e.blocks()]
        assert set(blocking) == {"busy-1@work", "busy-2@work"}

    def test_malformed_ics_returns_empty(self):
        assert parse_events("not a calendar", "x", WINDOW) == []


class TestConsolidation:
    def _combined_provider(self) -> FakeProvider:
        work = parse_events(read_fixture("work.ics"), "work", WINDOW)
        personal = parse_events(read_fixture("personal.ics"), "personal", WINDOW)
        return FakeProvider(work + personal)

    def test_duplicate_uid_deduped(self):
        provider = self._combined_provider()
        listed = provider.list_events(WINDOW)
        busy_1 = [e for e in listed if e.uid == "busy-1@work"]
        assert len(busy_1) == 1  # same UID across two calendars -> one entry

    def test_cross_calendar_sources_present(self):
        provider = self._combined_provider()
        sources = {e.source for e in provider.list_events(WINDOW)}
        assert sources == {"work", "personal"}


class TestBuildVevent:
    def test_booking_ics_has_attendee_and_organizer(self):
        from scheduler.providers.base import Event
        from scheduler.providers.caldav_provider import build_vevent

        ev = Event(
            uid="book-1",
            start=utc(2026, 7, 2, 11),
            end=utc(2026, 7, 2, 12),
            summary="Intro call",
            source="bookings",
            event_type="busy",
            transparent=False,
        )
        ics = build_vevent(ev, "alice@example.com", ["Sam <sam@example.com>"])
        assert "METHOD:REQUEST" in ics
        assert "ORGANIZER" in ics and "alice@example.com" in ics
        assert "mailto:sam@example.com" in ics
        # Round-trips back into a parseable event.
        parsed = parse_events(ics, "bookings", WINDOW)
        assert parsed[0].uid == "book-1"
