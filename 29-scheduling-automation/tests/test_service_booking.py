"""Service-level free-slot and booking tests through the fake provider."""

from datetime import UTC, datetime, time, timedelta

import pytest

from scheduler.config import default_availability
from scheduler.domain.availability import AvailabilityRule, WeeklyAvailability
from scheduler.domain.clock import FixedClock
from scheduler.domain.interval import Interval
from scheduler.domain.slots import SlotRequest
from scheduler.providers.fake_provider import FakeProvider
from scheduler.providers.ics import parse_events
from scheduler.service import (
    InvalidBookingError,
    SchedulingService,
    SlotTakenError,
)

from .conftest import read_fixture


def utc(y, mo, d, h=0, m=0):
    return datetime(y, mo, d, h, m, tzinfo=UTC)


# Mon-Fri 09:00-17:00 UTC. 2026-07-02 is a Thursday.
AVAIL = WeeklyAvailability(
    tz="UTC",
    rules=tuple(AvailabilityRule(d, time(9), time(17)) for d in range(5)),
)
WINDOW = Interval(utc(2026, 7, 2), utc(2026, 7, 3))
NOW = FixedClock(utc(2026, 7, 1))


def make_service(fail_create_once=False, extra_events=None):
    events = parse_events(read_fixture("work.ics"), "work", WINDOW)
    events += parse_events(read_fixture("personal.ics"), "personal", WINDOW)
    if extra_events:
        events += extra_events
    provider = FakeProvider(events, fail_create_once=fail_create_once)
    service = SchedulingService(
        provider=provider,
        availability=AVAIL,
        clock=NOW,
        organizer_email="alice@example.com",
    )
    return service, provider


class TestFreeSlots:
    def test_end_to_end_matches_hand_computed(self):
        service, _ = make_service()
        req = SlotRequest(
            duration=timedelta(hours=1), window=WINDOW, step=timedelta(hours=1)
        )
        slots = service.get_free_slots(req)
        # Busy mask: 09-10 (standup), 14-16 (review + dentist merged).
        # Availability 09-17 minus busy => free 10-14 and 16-17.
        starts = [s.start.hour for s in slots]
        assert starts == [10, 11, 12, 13, 16]

    def test_busy_intervals_merged_across_calendars(self):
        service, _ = make_service()
        busy = service.get_busy_intervals(WINDOW)
        assert Interval(utc(2026, 7, 2, 9), utc(2026, 7, 2, 10)) in busy
        assert Interval(utc(2026, 7, 2, 14), utc(2026, 7, 2, 16)) in busy
        assert len(busy) == 2


class TestBookingSuccess:
    def test_book_creates_and_is_listed(self):
        service, provider = make_service()
        slot = Interval(utc(2026, 7, 2, 11), utc(2026, 7, 2, 12))
        uid = service.book(slot, "Sam", "sam@example.com", "Intro call")
        assert uid
        assert len(provider.created) == 1
        listed_uids = {e.uid for e in provider.list_events(WINDOW)}
        assert uid in listed_uids

    def test_booked_slot_blocks_future_slots(self):
        service, _ = make_service()
        slot = Interval(utc(2026, 7, 2, 11), utc(2026, 7, 2, 12))
        service.book(slot, "Sam", "sam@example.com")
        req = SlotRequest(
            duration=timedelta(hours=1), window=WINDOW, step=timedelta(hours=1)
        )
        starts = [s.start.hour for s in service.get_free_slots(req)]
        assert 11 not in starts


class TestBookingRaces:
    def test_conditional_create_conflict_returns_409(self):
        service, provider = make_service(fail_create_once=True)
        slot = Interval(utc(2026, 7, 2, 11), utc(2026, 7, 2, 12))
        with pytest.raises(SlotTakenError):
            service.book(slot, "Sam", "sam@example.com")
        assert provider.created == []

    def test_slot_made_busy_after_listing_revalidates(self):
        service, provider = make_service()
        # 09:00 is already busy (standup); re-validation must reject it.
        slot = Interval(utc(2026, 7, 2, 9), utc(2026, 7, 2, 10))
        with pytest.raises(SlotTakenError):
            service.book(slot, "Sam", "sam@example.com")
        assert provider.created == []


class TestBookingValidation:
    def test_malformed_email_rejected_before_write(self):
        service, provider = make_service()
        slot = Interval(utc(2026, 7, 2, 11), utc(2026, 7, 2, 12))
        with pytest.raises(InvalidBookingError):
            service.book(slot, "Sam", "not-an-email")
        assert provider.created == []

    def test_min_notice_rejected_before_write(self):
        service, provider = make_service()
        # now = 2026-07-01; slot at +12h with a 30-day notice requirement.
        slot = Interval(utc(2026, 7, 2, 11), utc(2026, 7, 2, 12))
        with pytest.raises(InvalidBookingError):
            service.book(
                slot, "Sam", "sam@example.com", min_notice=timedelta(days=30)
            )
        assert provider.created == []

    def test_past_slot_rejected(self):
        service, provider = make_service()
        past = Interval(utc(2026, 6, 1, 11), utc(2026, 6, 1, 12))
        with pytest.raises(InvalidBookingError):
            service.book(past, "Sam", "sam@example.com")
        assert provider.created == []


def test_default_availability_helper():
    avail = default_availability("Europe/London")
    assert avail.tz == "Europe/London"
    assert {r.weekday for r in avail.rules} == {0, 1, 2, 3, 4}
