"""Unit tests for the core free-slot engine."""

from datetime import UTC, datetime, time, timedelta

import pytest

from scheduler.domain.availability import AvailabilityRule, WeeklyAvailability
from scheduler.domain.interval import Interval
from scheduler.domain.slots import SlotRequest, free_slots


def u(h, m=0, day=1):
    """2026-07-`day` UTC datetime (July 1 2026 is a Wednesday)."""
    return datetime(2026, 7, day, h, m, tzinfo=UTC)


# Wednesday is weekday 2. A single 09:00-17:00 UTC window on that Wednesday.
WED = 2
AVAIL = WeeklyAvailability(
    tz="UTC",
    rules=(AvailabilityRule(weekday=WED, start=time(9, 0), end=time(17, 0)),),
)
DAY = Interval(u(0), u(0, day=2))
PAST = u(0, day=1) - timedelta(days=365)  # well before the window


def starts(slots):
    """Return the (hour, minute) of each slot start for compact assertions."""
    return [(s.start.hour, s.start.minute) for s in slots]


class TestEmptyBusy:
    def test_full_window_sliced(self):
        req = SlotRequest(
            duration=timedelta(minutes=30),
            window=DAY,
            step=timedelta(minutes=15),
        )
        slots = free_slots(AVAIL, [], req, PAST)
        # 09:00 .. 16:30 inclusive, stepped by 15 minutes => 31 slots.
        assert len(slots) == 31
        assert slots[0] == Interval(u(9), u(9, 30))
        assert slots[-1] == Interval(u(16, 30), u(17))

    def test_duration_longer_than_window_is_empty(self):
        req = SlotRequest(duration=timedelta(hours=9), window=DAY)
        assert free_slots(AVAIL, [], req, PAST) == []


class TestBusySplitting:
    def test_one_meeting_splits_window(self):
        req = SlotRequest(
            duration=timedelta(hours=1),
            window=DAY,
            step=timedelta(hours=1),
        )
        busy = [Interval(u(12), u(13))]
        slots = free_slots(AVAIL, busy, req, PAST)
        assert starts(slots) == [
            (9, 0), (10, 0), (11, 0),  # before
            (13, 0), (14, 0), (15, 0), (16, 0),  # after
        ]

    def test_fully_booked_is_empty(self):
        req = SlotRequest(duration=timedelta(minutes=30), window=DAY)
        busy = [Interval(u(8), u(18))]
        assert free_slots(AVAIL, busy, req, PAST) == []

    def test_abutting_busy_no_spurious_gap(self):
        # Meeting exactly at the window edge does not remove interior slots.
        req = SlotRequest(
            duration=timedelta(hours=1), window=DAY, step=timedelta(hours=1)
        )
        busy = [Interval(u(8), u(9))]  # abuts 09:00 start
        slots = free_slots(AVAIL, busy, req, PAST)
        assert starts(slots)[0] == (9, 0)


class TestMinNotice:
    def test_trims_slots_before_notice(self):
        # now = day start; min_notice pushes the floor to 13:00.
        now = u(0)
        req = SlotRequest(
            duration=timedelta(hours=1),
            window=DAY,
            step=timedelta(hours=1),
            min_notice=timedelta(hours=13),
        )
        slots = free_slots(AVAIL, [], req, now)
        assert starts(slots) == [(13, 0), (14, 0), (15, 0), (16, 0)]

    def test_partial_interval_trimmed_to_floor(self):
        now = u(9, 20)
        req = SlotRequest(
            duration=timedelta(minutes=30),
            window=DAY,
            step=timedelta(minutes=30),
            min_notice=timedelta(0),
        )
        slots = free_slots(AVAIL, [], req, now)
        # First slot starts no earlier than 09:20.
        assert slots[0].start >= now


class TestBuffers:
    def test_buffers_expand_busy(self):
        req = SlotRequest(
            duration=timedelta(hours=1),
            window=DAY,
            step=timedelta(hours=1),
            buffer_before=timedelta(minutes=30),
            buffer_after=timedelta(minutes=30),
        )
        busy = [Interval(u(12), u(13))]  # blocks 11:30..13:30 with buffers
        slots = free_slots(AVAIL, busy, req, PAST)
        # Free is 09:00-11:30 and 13:30-17:00; 1h slots stepped hourly from each edge.
        assert starts(slots) == [(9, 0), (10, 0), (13, 30), (14, 30), (15, 30)]
        # A 1h slot at 11:00 would spill into the buffer, so it is excluded.
        assert (11, 0) not in starts(slots)


class TestStepVariations:
    def test_step_smaller_than_duration_overlaps(self):
        req = SlotRequest(
            duration=timedelta(hours=1),
            window=Interval(u(9), u(11)),
            step=timedelta(minutes=30),
        )
        avail = WeeklyAvailability(
            tz="UTC",
            rules=(AvailabilityRule(WED, time(9), time(11)),),
        )
        slots = free_slots(avail, [], req, PAST)
        assert starts(slots) == [(9, 0), (9, 30), (10, 0)]

    def test_step_equals_duration_back_to_back(self):
        req = SlotRequest(
            duration=timedelta(hours=1),
            window=Interval(u(9), u(12)),
            step=timedelta(hours=1),
        )
        avail = WeeklyAvailability(
            tz="UTC", rules=(AvailabilityRule(WED, time(9), time(12)),)
        )
        slots = free_slots(avail, [], req, PAST)
        assert starts(slots) == [(9, 0), (10, 0), (11, 0)]

    def test_step_larger_than_duration_leaves_gaps(self):
        req = SlotRequest(
            duration=timedelta(minutes=30),
            window=Interval(u(9), u(12)),
            step=timedelta(hours=1),
        )
        avail = WeeklyAvailability(
            tz="UTC", rules=(AvailabilityRule(WED, time(9), time(12)),)
        )
        slots = free_slots(avail, [], req, PAST)
        assert starts(slots) == [(9, 0), (10, 0), (11, 0)]
        assert all(s.duration == timedelta(minutes=30) for s in slots)


class TestSlotRequestValidation:
    def test_zero_duration_rejected(self):
        with pytest.raises(ValueError):
            SlotRequest(duration=timedelta(0), window=DAY)

    def test_negative_step_rejected(self):
        with pytest.raises(ValueError):
            SlotRequest(duration=timedelta(minutes=30), window=DAY, step=timedelta(-1))

    def test_negative_buffer_rejected(self):
        with pytest.raises(ValueError):
            SlotRequest(
                duration=timedelta(minutes=30),
                window=DAY,
                buffer_before=timedelta(minutes=-5),
            )


class TestEmptyAvailability:
    def test_no_rules_returns_empty(self):
        empty = WeeklyAvailability(tz="UTC", rules=())
        req = SlotRequest(duration=timedelta(minutes=30), window=DAY)
        assert free_slots(empty, [], req, PAST) == []
