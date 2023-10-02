"""In-memory calendar provider for tests and offline demos.

It can be seeded from ICS through the same parser as the CalDAV provider, and
can simulate losing a booking race.
"""

from __future__ import annotations

import uuid
from datetime import UTC

from ..domain.interval import Interval
from .base import CalendarProvider, ConflictError, Event
from .ics import parse_events


class FakeProvider(CalendarProvider):
    """In-memory provider.

    With ``fail_create_once`` the next create_event raises ConflictError, as
    if another booking had won the race.
    """

    def __init__(
        self,
        events: list[Event] | None = None,
        *,
        fail_create_once: bool = False,
    ) -> None:
        self._events: list[Event] = list(events or [])
        self._fail_create_once = fail_create_once
        self.created: list[tuple[Event, list[str]]] = []

    @classmethod
    def from_ics(
        cls,
        ics_text: str | bytes,
        source: str,
        window: Interval,
        organizer_tz: str = "UTC",
    ) -> FakeProvider:
        """Seed from ICS, expanding recurrence over *window*."""
        return cls(parse_events(ics_text, source, window, organizer_tz))

    def add_event(self, event: Event) -> None:
        self._events.append(event)

    def list_events(self, window: Interval) -> list[Event]:
        """Events overlapping *window*. The same UID in several calendars is
        one event, so it is only returned once."""
        seen: set[str] = set()
        result: list[Event] = []
        for ev in self._events:
            if not ev.to_interval().overlaps(window):
                continue
            if ev.uid and ev.uid in seen:
                continue
            if ev.uid:
                seen.add(ev.uid)
            result.append(ev)
        return sorted(result, key=lambda e: (e.start, e.uid))

    def create_event(self, ev: Event, attendees: list[str]) -> str:
        """Store *ev* and return its UID. Raises ConflictError if the slot is
        busy or a lost race is being simulated."""
        if self._fail_create_once:
            self._fail_create_once = False
            raise ConflictError("Simulated conditional-create conflict")

        slot = ev.to_interval()
        for existing in self._events:
            if existing.blocks() and existing.to_interval().overlaps(slot):
                raise ConflictError(f"Slot {slot.start}..{slot.end} already busy")

        uid = ev.uid or str(uuid.uuid4())
        stored = Event(
            uid=uid,
            start=ev.start.astimezone(UTC),
            end=ev.end.astimezone(UTC),
            summary=ev.summary,
            source=ev.source,
            event_type=ev.event_type,
            transparent=ev.transparent,
        )
        self._events.append(stored)
        self.created.append((stored, attendees))
        return uid
