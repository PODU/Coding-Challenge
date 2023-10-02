"""The calendar provider interface.

Providers return plain UTC :class:`Event` objects, so the rest of the code
never has to deal with iCalendar, odd timezones or recurrence rules.
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from typing import Protocol, runtime_checkable

from ..domain.interval import Interval


class ProviderError(Exception):
    """A calendar backend failed (network, auth, ...)."""


class ConflictError(ProviderError):
    """A conditional create found the slot already taken."""


@dataclass(frozen=True)
class Event:
    """A calendar event with UTC bounds."""

    uid: str
    start: datetime
    end: datetime
    summary: str
    source: str  # calendar name
    event_type: str  # "busy", "free", "tentative" or "all_day"
    transparent: bool  # TRANSP:TRANSPARENT

    def blocks(self) -> bool:
        """Whether the event makes its time unavailable."""
        return not self.transparent and self.event_type != "free"

    def to_interval(self) -> Interval:
        return Interval(self.start, self.end)


@runtime_checkable
class CalendarProvider(Protocol):

    def list_events(self, window: Interval) -> list[Event]:
        """Events overlapping *window* from every configured calendar, with
        recurring events expanded."""
        ...

    def create_event(self, ev: Event, attendees: list[str]) -> str:
        """Create *ev*, invite *attendees* and return the UID.

        Should raise :class:`ConflictError` if someone else got the slot first.
        """
        ...
