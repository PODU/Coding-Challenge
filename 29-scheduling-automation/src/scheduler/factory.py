"""Builds the real, CalDAV-backed service from settings."""

from __future__ import annotations

from .config import Settings, default_availability, parse_availability_spec
from .domain.clock import SystemClock
from .providers.auth import BasicCredentials, OAuthToken
from .providers.base import ProviderError
from .providers.caldav_provider import CalDAVProvider
from .service import SchedulingService


def build_service(
    settings: Settings | None = None,
    availability_spec: str | None = None,
) -> SchedulingService:
    """Build the service from *settings* (default: the environment).

    Availability defaults to Mon-Fri 9-5. Raises ProviderError if there are no
    credentials of either kind.
    """
    settings = settings or Settings.from_env()

    credentials = None
    oauth = None
    if settings.username and settings.app_password:
        credentials = BasicCredentials(settings.username, settings.app_password)
    elif settings.oauth_token_json:
        oauth = OAuthToken.from_json(settings.oauth_token_json)
    else:
        raise ProviderError("No CalDAV credentials configured (set basic auth or OAuth)")

    provider = CalDAVProvider(
        url=settings.caldav_url,
        organizer_email=settings.organizer_email,
        organizer_tz=settings.organizer_tz,
        calendars=list(settings.calendars),
        credentials=credentials,
        oauth=oauth,
    )

    if availability_spec:
        availability = parse_availability_spec(availability_spec, settings.organizer_tz)
    else:
        availability = default_availability(settings.organizer_tz)

    return SchedulingService(
        provider=provider,
        availability=availability,
        clock=SystemClock(),
        organizer_email=settings.organizer_email,
    )
