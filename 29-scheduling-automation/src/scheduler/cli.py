"""Command-line interface.

* ``events``: print busy times across all calendars
* ``slots``: print free slots for a duration and window
* ``book``: book a slot and invite an attendee

Exit codes: 0 ok, 2 bad arguments, 3 slot no longer free, 4 provider or
auth error.
"""

from __future__ import annotations

import argparse
import re
import sys
from datetime import UTC, datetime, timedelta

from .domain.interval import Interval
from .domain.slots import SlotRequest
from .providers.auth import AuthError
from .providers.base import ProviderError
from .service import InvalidBookingError, SchedulingService, SlotTakenError

EXIT_OK = 0
EXIT_BAD_ARGS = 2
EXIT_SLOT_TAKEN = 3
EXIT_PROVIDER = 4

_DURATION_RE = re.compile(r"^(?:(\d+)h)?(?:(\d+)m)?$")


def parse_duration(text: str) -> timedelta:
    """Parse ``"30m"``, ``"1h"`` or ``"1h30m"``. Raises ValueError otherwise."""
    match = _DURATION_RE.match(text.strip())
    if not match or text.strip() == "":
        raise ValueError(f"Invalid duration: {text!r}")
    total = timedelta(hours=int(match.group(1) or 0), minutes=int(match.group(2) or 0))
    if total <= timedelta(0):
        raise ValueError(f"Duration must be positive: {text!r}")
    return total


def parse_datetime(text: str) -> datetime:
    """Parse ISO-8601, accepting a ``Z`` suffix. Naive values are taken as UTC."""
    value = datetime.fromisoformat(text.replace("Z", "+00:00"))
    if value.tzinfo is None:
        value = value.replace(tzinfo=UTC)
    return value


def parse_date_bound(text: str) -> datetime:
    """Like parse_datetime, but a bare date means midnight UTC."""
    try:
        return parse_datetime(text)
    except ValueError:
        return datetime.fromisoformat(text).replace(tzinfo=UTC)


def _split_attendee(raw: str) -> tuple[str, str]:
    """Split ``"Name <email>"`` or a bare email into (name, email)."""
    if "<" in raw and ">" in raw:
        name = raw.split("<")[0].strip()
        email = raw.split("<")[1].split(">")[0].strip()
        return name, email
    return "", raw.strip()


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="scheduler", description="Scheduling automation")
    sub = parser.add_subparsers(dest="command", required=True)

    p_events = sub.add_parser("events", help="List consolidated busy events")
    p_events.add_argument("--from", dest="from_", required=True)
    p_events.add_argument("--to", dest="to", required=True)

    p_slots = sub.add_parser("slots", help="List free slots")
    p_slots.add_argument("--duration", required=True)
    p_slots.add_argument("--from", dest="from_", required=True)
    p_slots.add_argument("--to", dest="to", required=True)
    p_slots.add_argument("--step", default="15m")

    p_book = sub.add_parser("book", help="Book a slot")
    p_book.add_argument("--start", required=True)
    p_book.add_argument("--duration", required=True)
    p_book.add_argument("--attendee", required=True)
    p_book.add_argument("--title", default=None)

    return parser


def _run(args: argparse.Namespace, service: SchedulingService, out) -> int:
    if args.command == "events":
        window = Interval(parse_date_bound(args.from_), parse_date_bound(args.to))
        for interval in service.get_busy_intervals(window):
            out.write(f"{interval.start.isoformat()}  {interval.end.isoformat()}\n")
        return EXIT_OK

    if args.command == "slots":
        window = Interval(parse_date_bound(args.from_), parse_date_bound(args.to))
        req = SlotRequest(
            duration=parse_duration(args.duration),
            window=window,
            step=parse_duration(args.step),
        )
        for slot in service.get_free_slots(req):
            out.write(f"{slot.start.isoformat()}  {slot.end.isoformat()}\n")
        return EXIT_OK

    if args.command == "book":
        start = parse_datetime(args.start)
        end = start + parse_duration(args.duration)
        name, email = _split_attendee(args.attendee)
        try:
            uid = service.book(Interval(start, end), name, email, args.title)
        except InvalidBookingError as exc:
            out.write(f"error: {exc}\n")
            return EXIT_BAD_ARGS
        except SlotTakenError as exc:
            out.write(f"error: {exc}\n")
            return EXIT_SLOT_TAKEN
        out.write(f"booked: {uid}\n")
        return EXIT_OK

    return EXIT_BAD_ARGS  # pragma: no cover - argparse enforces a command


def main(
    argv: list[str] | None = None,
    service: SchedulingService | None = None,
    out=None,
) -> int:
    """Run the CLI and return the exit code.

    Without a *service*, a CalDAV-backed one is built from the environment.
    """
    out = out or sys.stdout
    parser = build_parser()
    try:
        args = parser.parse_args(argv)
    except SystemExit as exc:
        return EXIT_BAD_ARGS if exc.code not in (0, None) else EXIT_OK

    try:
        if service is None:
            from .factory import build_service

            service = build_service()
        return _run(args, service, out)
    except (ValueError, argparse.ArgumentError) as exc:
        out.write(f"error: {exc}\n")
        return EXIT_BAD_ARGS
    except SlotTakenError as exc:
        out.write(f"error: {exc}\n")
        return EXIT_SLOT_TAKEN
    except (ProviderError, AuthError) as exc:
        out.write(f"error: {exc}\n")
        return EXIT_PROVIDER


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
