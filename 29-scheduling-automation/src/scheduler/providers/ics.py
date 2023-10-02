"""Turn ICS text into UTC :class:`Event` objects.

Recurrence is expanded within a window, all-day and floating times are placed
in the organizer's timezone, and cancelled events are dropped. Used by both
the CalDAV and the fake provider.
"""

from __future__ import annotations

import logging
from datetime import UTC, date, datetime, timedelta
from zoneinfo import ZoneInfo

import recurring_ical_events
from icalendar import Calendar

from ..domain.interval import Interval
from .base import Event

logger = logging.getLogger(__name__)


def _to_utc(value: datetime | date, organizer_tz: ZoneInfo) -> datetime:
    """Convert to UTC. Dates (all-day) become local midnight, and naive
    (floating) times are read as local, in the organizer's timezone."""
    if isinstance(value, datetime):
        if value.tzinfo is None:
            value = value.replace(tzinfo=organizer_tz)
        return value.astimezone(UTC)
    return datetime(value.year, value.month, value.day, tzinfo=organizer_tz).astimezone(UTC)


def _component_end(component, start_utc: datetime, organizer_tz: ZoneInfo) -> datetime:
    """End of a VEVENT, from DTEND or DURATION."""
    dtend = component.get("DTEND")
    if dtend is not None:
        return _to_utc(dtend.dt, organizer_tz)
    duration = component.get("DURATION")
    if duration is not None:
        return start_utc + duration.dt
    # Neither given: assume an hour for a timed event and a day for an all-day one.
    raw_start = component.get("DTSTART").dt
    if isinstance(raw_start, datetime):
        return start_utc + timedelta(hours=1)
    return start_utc + timedelta(days=1)


def _classify(component) -> tuple[str, bool]:
    """Return ``(event_type, transparent)`` for a VEVENT."""
    raw_start = component.get("DTSTART").dt
    transparent = str(component.get("TRANSP", "OPAQUE")).upper() == "TRANSPARENT"

    if not isinstance(raw_start, datetime):
        event_type = "all_day"
    else:
        status = str(component.get("STATUS", "")).upper()
        if transparent:
            event_type = "free"
        elif status == "TENTATIVE":
            event_type = "tentative"
        else:
            event_type = "busy"
    return event_type, transparent


def parse_events(
    ics_text: str | bytes,
    source: str,
    window: Interval,
    organizer_tz: str = "UTC",
) -> list[Event]:
    """Parse an ICS document into events overlapping *window*.

    Events are tagged with *source* (the calendar name). A malformed event is
    logged and skipped rather than failing the whole calendar.
    """
    tz = ZoneInfo(organizer_tz)
    try:
        calendar = Calendar.from_ical(ics_text)
    except Exception as exc:  # noqa: BLE001 - servers send all sorts of junk
        logger.warning("Failed to parse calendar from %s: %s", source, exc)
        return []

    try:
        occurrences = recurring_ical_events.of(calendar).between(
            window.start, window.end
        )
    except Exception as exc:  # noqa: BLE001
        logger.warning("Recurrence expansion failed for %s: %s", source, exc)
        return []

    events: list[Event] = []
    for component in occurrences:
        if component.name != "VEVENT":
            continue
        try:
            status = str(component.get("STATUS", "")).upper()
            if status == "CANCELLED":
                continue
            start_utc = _to_utc(component.get("DTSTART").dt, tz)
            end_utc = _component_end(component, start_utc, tz)
            if end_utc <= start_utc:
                logger.warning("Skipping zero/negative-length event in %s", source)
                continue
            event_type, transparent = _classify(component)
            events.append(
                Event(
                    uid=str(component.get("UID", "")),
                    start=start_utc,
                    end=end_utc,
                    summary=str(component.get("SUMMARY", "")),
                    source=source,
                    event_type=event_type,
                    transparent=transparent,
                )
            )
        except Exception as exc:  # noqa: BLE001 - skip one bad event, keep going
            logger.warning("Skipping malformed event in %s: %s", source, exc)
    return events
