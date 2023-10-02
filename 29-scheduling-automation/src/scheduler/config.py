"""Settings read from the environment, including credentials."""

from __future__ import annotations

import os
from dataclasses import dataclass, field
from datetime import time

from .domain.availability import AvailabilityRule, WeeklyAvailability


@dataclass(frozen=True)
class Settings:
    caldav_url: str = ""
    username: str = ""
    app_password: str = ""
    oauth_client_id: str = ""
    oauth_client_secret: str = ""
    oauth_token_json: str = ""
    calendars: tuple[str, ...] = field(default_factory=tuple)  # empty = all
    organizer_tz: str = "UTC"
    organizer_email: str = ""

    @classmethod
    def from_env(cls, environ: dict[str, str] | None = None) -> Settings:
        env = os.environ if environ is None else environ
        calendars = tuple(c.strip() for c in env.get("CALENDARS", "").split(",") if c.strip())
        return cls(
            caldav_url=env.get("CALDAV_URL", ""),
            username=env.get("CALDAV_USERNAME", ""),
            app_password=env.get("CALDAV_APP_PASSWORD", ""),
            oauth_client_id=env.get("OAUTH_CLIENT_ID", ""),
            oauth_client_secret=env.get("OAUTH_CLIENT_SECRET", ""),
            oauth_token_json=env.get("OAUTH_TOKEN_JSON", ""),
            calendars=calendars,
            organizer_tz=env.get("ORGANIZER_TZ", "UTC"),
            organizer_email=env.get("ORGANIZER_EMAIL", ""),
        )


def default_availability(tz: str = "UTC") -> WeeklyAvailability:
    """Mon-Fri, 09:00-17:00 in *tz*."""
    rules = tuple(
        AvailabilityRule(weekday=day, start=time(9, 0), end=time(17, 0))
        for day in range(5)
    )
    return WeeklyAvailability(tz=tz, rules=rules)


def parse_availability_spec(spec: str, tz: str) -> WeeklyAvailability:
    """Parse a spec like ``"Mon-Fri 09:00-17:00, Sat 10:00-12:00"``.

    Each comma-separated clause is a day or ``Day-Day`` range (three-letter
    English names) and a time range. Raises ValueError on bad input.
    """
    names = ["mon", "tue", "wed", "thu", "fri", "sat", "sun"]
    rules: list[AvailabilityRule] = []
    for clause in spec.split(","):
        clause = clause.strip()
        if not clause:
            continue
        try:
            days_part, times_part = clause.split()
            start_s, end_s = times_part.split("-")
            start_t = time.fromisoformat(start_s)
            end_t = time.fromisoformat(end_s)
        except ValueError as exc:
            raise ValueError(f"Malformed availability clause: {clause!r}") from exc

        if "-" in days_part:
            a, b = days_part.lower().split("-")
            lo, hi = names.index(a[:3]), names.index(b[:3])
            day_range = range(lo, hi + 1)
        else:
            day_range = [names.index(days_part.lower()[:3])]
        for day in day_range:
            rules.append(AvailabilityRule(weekday=day, start=start_t, end=end_t))
    if not rules:
        raise ValueError(f"No rules parsed from spec: {spec!r}")
    return WeeklyAvailability(tz=tz, rules=tuple(rules))
