"""CalDAV calendar provider.

ICS parsing and recurrence live in :mod:`scheduler.providers.ics`. The
``caldav`` package is imported lazily so the rest of the code (and the tests)
work without it.
"""

from __future__ import annotations

import logging
import uuid
from datetime import UTC, datetime

from ..domain.interval import Interval
from .auth import BasicCredentials, OAuthToken
from .base import CalendarProvider, ConflictError, Event, ProviderError
from .ics import parse_events

logger = logging.getLogger(__name__)


def build_vevent(ev: Event, organizer_email: str, attendees: list[str]) -> str:
    """Build the VCALENDAR for a booking.

    *attendees* are ``"Name <email>"`` or bare emails. ``METHOD:REQUEST``
    makes the server send out the invitations.
    """
    from icalendar import Calendar, vCalAddress, vText
    from icalendar import Event as ICalEvent

    cal = Calendar()
    cal.add("prodid", "-//scheduler//coding-challenge-29//EN")
    cal.add("version", "2.0")
    cal.add("method", "REQUEST")

    vevent = ICalEvent()
    vevent.add("uid", ev.uid or str(uuid.uuid4()))
    vevent.add("dtstamp", datetime.now(UTC))
    vevent.add("dtstart", ev.start.astimezone(UTC))
    vevent.add("dtend", ev.end.astimezone(UTC))
    vevent.add("summary", ev.summary)

    organizer = vCalAddress(f"mailto:{organizer_email}")
    organizer.params["cn"] = vText(organizer_email)
    vevent.add("organizer", organizer)

    for raw in attendees:
        email = raw.split("<")[-1].strip(">").strip() if "<" in raw else raw.strip()
        attendee = vCalAddress(f"mailto:{email}")
        attendee.params["role"] = vText("REQ-PARTICIPANT")
        attendee.params["rsvp"] = vText("TRUE")
        if "<" in raw:
            attendee.params["cn"] = vText(raw.split("<")[0].strip())
        vevent.add("attendee", attendee)

    cal.add_component(vevent)
    return cal.to_ical().decode("utf-8")


class CalDAVProvider(CalendarProvider):
    """Reads and books events over CalDAV.

    *calendars* limits which calendars (by display name) are used; empty means
    all of them. *organizer_tz* is used for all-day and floating times. Tests
    can pass a ready-made *client*.
    """

    def __init__(
        self,
        url: str,
        organizer_email: str,
        organizer_tz: str = "UTC",
        calendars: list[str] | None = None,
        credentials: BasicCredentials | None = None,
        oauth: OAuthToken | None = None,
        client=None,
    ) -> None:
        self.url = url
        self.organizer_email = organizer_email
        self.organizer_tz = organizer_tz
        self.calendar_names = calendars or []
        self._credentials = credentials
        self._oauth = oauth
        self._client = client
        if client is None and credentials is None and oauth is None:
            raise ProviderError("CalDAVProvider requires credentials or an OAuth token")

    def _connect(self):
        if self._client is not None:
            return self._client
        try:
            import caldav
        except ImportError as exc:  # pragma: no cover - depends on environment
            raise ProviderError("The 'caldav' package is required for live access") from exc

        kwargs: dict = {"url": self.url}
        if self._credentials is not None:
            kwargs["username"] = self._credentials.username
            kwargs["password"] = self._credentials.password
        elif self._oauth is not None:
            kwargs["headers"] = {"Authorization": self._oauth.authorization_header()}
        try:
            self._client = caldav.DAVClient(**kwargs)
        except Exception as exc:  # noqa: BLE001
            raise ProviderError(f"Failed to connect to CalDAV server: {exc}") from exc
        return self._client

    def _select_calendars(self):
        client = self._connect()
        try:
            calendars = client.principal().calendars()
        except Exception as exc:  # noqa: BLE001
            raise ProviderError(f"Failed to list calendars: {exc}") from exc

        if not self.calendar_names:
            return calendars
        wanted = set(self.calendar_names)
        selected = [c for c in calendars if str(c.name) in wanted]
        if not selected:
            logger.warning("No calendars matched %s; using all", self.calendar_names)
            return calendars
        return selected

    def list_events(self, window: Interval) -> list[Event]:
        """Events from every selected calendar, de-duplicated by UID."""
        events: list[Event] = []
        seen: set[str] = set()
        for calendar in self._select_calendars():
            source = str(getattr(calendar, "name", "") or "calendar")
            try:
                found = calendar.search(
                    start=window.start,
                    end=window.end,
                    event=True,
                    expand=False,
                )
            except Exception as exc:  # noqa: BLE001
                raise ProviderError(f"CalDAV REPORT failed for {source}: {exc}") from exc

            for obj in found:
                try:
                    ics_text = obj.data
                except Exception as exc:  # noqa: BLE001
                    logger.warning("Could not read object data from %s: %s", source, exc)
                    continue
                for ev in parse_events(ics_text, source, window, self.organizer_tz):
                    if ev.uid and ev.uid in seen:
                        continue
                    if ev.uid:
                        seen.add(ev.uid)
                    events.append(ev)
        return sorted(events, key=lambda e: (e.start, e.uid))

    def create_event(self, ev: Event, attendees: list[str]) -> str:
        """Create the event with ``no_overwrite`` (If-None-Match: *), so a
        competing booking can't silently replace it."""
        uid = ev.uid or str(uuid.uuid4())
        ev = Event(uid, ev.start, ev.end, ev.summary, ev.source, ev.event_type, ev.transparent)
        ics_text = build_vevent(ev, self.organizer_email, attendees)

        calendars = self._select_calendars()
        if not calendars:
            raise ProviderError("No target calendar available for booking")
        target = calendars[0]
        try:
            target.save_event(ics_text, no_overwrite=True)
        except Exception as exc:  # noqa: BLE001
            # caldav has no specific exception for this, so sniff the message.
            message = str(exc).lower()
            if any(m in message for m in ("exist", "412", "conflict", "precondition")):
                raise ConflictError(f"Slot already booked: {exc}") from exc
            raise ProviderError(f"Failed to create event: {exc}") from exc
        return uid
