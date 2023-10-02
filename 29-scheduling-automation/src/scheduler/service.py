"""The scheduling service used by both the CLI and the API."""

from __future__ import annotations

import re
import uuid
from datetime import UTC

from .domain.availability import WeeklyAvailability
from .domain.clock import Clock
from .domain.interval import Interval, merge
from .domain.slots import SlotRequest, free_slots
from .providers.base import CalendarProvider, ConflictError, Event

_EMAIL_RE = re.compile(r"^[^@\s]+@[^@\s]+\.[^@\s]+$")


class BookingError(Exception):
    """Base class for booking failures."""


class InvalidBookingError(BookingError):
    """Bad booking request (HTTP 400)."""


class SlotTakenError(BookingError):
    """The slot is no longer free (HTTP 409)."""


def _valid_email(email: str) -> bool:
    return bool(_EMAIL_RE.match(email.strip()))


class SchedulingService:

    def __init__(
        self,
        provider: CalendarProvider,
        availability: WeeklyAvailability,
        clock: Clock,
        organizer_email: str,
    ) -> None:
        self.provider = provider
        self.availability = availability
        self.clock = clock
        self.organizer_email = organizer_email

    def get_busy_intervals(self, window: Interval) -> list[Interval]:
        """Busy time within *window* across all calendars, merged.

        Transparent and "free" events don't count.
        """
        intervals: list[Interval] = []
        for event in self.provider.list_events(window):
            if not event.blocks():
                continue
            clipped = event.to_interval().clip(window)
            if clipped is not None:
                intervals.append(clipped)
        return merge(intervals)

    def get_free_slots(self, req: SlotRequest) -> list[Interval]:
        busy = self.get_busy_intervals(req.window)
        return free_slots(self.availability, busy, req, self.clock.now())

    def book(
        self,
        slot: Interval,
        attendee_name: str,
        attendee_email: str,
        title: str | None = None,
        min_notice=None,
    ) -> str:
        """Book *slot* and invite the attendee. Returns the new event's UID.

        Busy times are fetched again first, and the event is created
        conditionally, so two people can't book the same slot.

        Raises InvalidBookingError for a bad email or a slot that is too soon,
        and SlotTakenError if the slot is taken.
        """
        if not _valid_email(attendee_email):
            raise InvalidBookingError(f"Invalid attendee email: {attendee_email!r}")

        now = self.clock.now()
        if min_notice is not None and slot.start < now + min_notice:
            raise InvalidBookingError("Slot violates the minimum notice period")
        if slot.start < now:
            raise InvalidBookingError("Cannot book a slot in the past")

        # The caller's slot list may be stale.
        if any(block.overlaps(slot) for block in self.get_busy_intervals(slot)):
            raise SlotTakenError("Slot is no longer free")

        event = Event(
            uid=str(uuid.uuid4()),
            start=slot.start.astimezone(UTC),
            end=slot.end.astimezone(UTC),
            summary=title or f"Meeting with {attendee_name}",
            source="bookings",
            event_type="busy",
            transparent=False,
        )
        attendee = f"{attendee_name} <{attendee_email}>" if attendee_name else attendee_email
        try:
            return self.provider.create_event(event, [attendee])
        except ConflictError as exc:
            raise SlotTakenError(str(exc)) from exc
