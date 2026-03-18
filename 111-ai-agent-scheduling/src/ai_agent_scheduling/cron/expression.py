"""5-field cron parsing and next-fire calculation (no croniter dependency).

Fields::

    minute  hour  day-of-month  month  day-of-week

Each field accepts ``*``, ``*/n`` step, ``a-b`` range, ``a-b/n`` stepped range,
and comma-separated lists of any of those. Day-of-week is ``0-6`` (0 = Sunday);
``7`` is also accepted as Sunday. Standard cron day-of-month / day-of-week OR
semantics apply: when both are restricted, a match on *either* fires.
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime, timedelta
from typing import List

_FIELD_RANGES = {
    "minute": (0, 59),
    "hour": (0, 23),
    "dom": (1, 31),
    "month": (1, 12),
    "dow": (0, 6),
}

_MONTH_NAMES = {
    "jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
    "jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
}
_DOW_NAMES = {
    "sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6,
}


class CronError(ValueError):
    """Raised when a cron expression is syntactically or semantically invalid."""


def _parse_token(token: str, lo: int, hi: int, names: dict | None) -> set[int]:
    """Parse one comma-free token (``*``, ``a``, ``a-b``, ``*/n``, ``a-b/n``)."""
    names = names or {}

    step = 1
    if "/" in token:
        token, step_s = token.split("/", 1)
        if not step_s.isdigit() or int(step_s) == 0:
            raise CronError(f"invalid step '{step_s}'")
        step = int(step_s)

    if token == "*":
        start, end = lo, hi
    elif "-" in token:
        a_s, b_s = token.split("-", 1)
        start = _atom(a_s, names)
        end = _atom(b_s, names)
    else:
        v = _atom(token, names)
        start = end = v

    if start < lo or end > hi or start > end:
        raise CronError(f"value out of range: '{token}' (allowed {lo}-{hi})")

    return set(range(start, end + 1, step))


def _atom(s: str, names: dict) -> int:
    s = s.strip().lower()
    if s in names:
        return names[s]
    if not s.lstrip("-").isdigit():
        raise CronError(f"invalid value '{s}'")
    return int(s)


def _parse_field(field: str, name: str) -> set[int]:
    lo, hi = _FIELD_RANGES[name]
    names = _MONTH_NAMES if name == "month" else (_DOW_NAMES if name == "dow" else None)
    values: set[int] = set()
    for token in field.split(","):
        token = token.strip()
        if token == "":
            raise CronError("empty field component")
        if name == "dow" and token == "7":
            token = "0"  # 7 is an alias for Sunday
        values |= _parse_token(token, lo, hi, names)
    if not values:
        raise CronError(f"field '{name}' matched nothing")
    return values


@dataclass(frozen=True)
class CronExpression:
    """A validated 5-field cron expression with next-fire computation."""

    raw: str
    minutes: frozenset[int]
    hours: frozenset[int]
    doms: frozenset[int]
    months: frozenset[int]
    dows: frozenset[int]
    dom_restricted: bool
    dow_restricted: bool

    @classmethod
    def parse(cls, expr: str) -> "CronExpression":
        if expr is None:
            raise CronError("cron expression is required")
        parts = expr.strip().split()
        if len(parts) != 5:
            raise CronError(
                f"expected 5 fields (minute hour day-of-month month day-of-week), got {len(parts)}: {expr!r}"
            )
        minute, hour, dom, month, dow = parts
        return cls(
            raw=" ".join(parts),
            minutes=frozenset(_parse_field(minute, "minute")),
            hours=frozenset(_parse_field(hour, "hour")),
            doms=frozenset(_parse_field(dom, "dom")),
            months=frozenset(_parse_field(month, "month")),
            dows=frozenset(_parse_field(dow, "dow")),
            dom_restricted=dom.strip() != "*",
            dow_restricted=dow.strip() != "*",
        )

    @staticmethod
    def is_valid(expr: str) -> bool:
        try:
            CronExpression.parse(expr)
            return True
        except CronError:
            return False

    def _day_matches(self, dt: datetime) -> bool:
        # Python's weekday() is Monday=0; cron uses Sunday=0.
        dow = (dt.weekday() + 1) % 7
        dom_ok = dt.day in self.doms
        dow_ok = dow in self.dows
        if self.dom_restricted and self.dow_restricted:
            return dom_ok or dow_ok
        if self.dom_restricted:
            return dom_ok
        if self.dow_restricted:
            return dow_ok
        return True

    def matches(self, dt: datetime) -> bool:
        return (
            dt.minute in self.minutes
            and dt.hour in self.hours
            and dt.month in self.months
            and self._day_matches(dt)
        )

    def next_fire(self, after: datetime, *, inclusive: bool = False) -> datetime:
        """Return the first matching minute after ``after``.

        With ``inclusive=True`` the minute containing ``after`` also counts. The
        search skips whole months, days and hours that cannot match, so even
        yearly schedules take only a few iterations. tzinfo is preserved.
        """
        candidate = after.replace(second=0, microsecond=0)
        if not inclusive:
            candidate += timedelta(minutes=1)

        # Bound the search to ~4 years to guarantee termination on impossible
        # specs (e.g. Feb 30); a valid expression always resolves far sooner.
        limit = after + timedelta(days=366 * 4)
        while candidate <= limit:
            if candidate.month not in self.months:
                candidate = _start_of_next_month(candidate)
                continue
            if not self._day_matches(candidate):
                candidate = _start_of_next_day(candidate)
                continue
            if candidate.hour not in self.hours:
                candidate = candidate.replace(minute=0) + timedelta(hours=1)
                continue
            if candidate.minute not in self.minutes:
                candidate += timedelta(minutes=1)
                continue
            return candidate
        raise CronError(f"no fire time found within 4 years for {self.raw!r}")


def _start_of_next_day(dt: datetime) -> datetime:
    return (dt + timedelta(days=1)).replace(hour=0, minute=0)


def _start_of_next_month(dt: datetime) -> datetime:
    if dt.month == 12:
        return dt.replace(year=dt.year + 1, month=1, day=1, hour=0, minute=0)
    return dt.replace(month=dt.month + 1, day=1, hour=0, minute=0)


_DOW_LABELS = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"]
_MONTH_LABELS = [
    "", "January", "February", "March", "April", "May", "June",
    "July", "August", "September", "October", "November", "December",
]


def describe(expr: str) -> str:
    """Return a one-line, human-readable description of a cron expression."""
    parts = expr.strip().split()
    if len(parts) != 5:
        return expr
    minute, hour, dom, month, dow = parts

    if minute.startswith("*/") and hour == "*":
        time_clause = f"every {minute[2:]} minute(s)"
    elif minute == "*" and hour == "*":
        time_clause = "every minute"
    elif hour.startswith("*/") and minute.isdigit():
        time_clause = f"every {hour[2:]} hour(s) at minute {int(minute)}"
    elif hour == "*" and minute.isdigit():
        time_clause = f"every hour at minute {int(minute)}"
    elif minute.isdigit() and hour.isdigit():
        time_clause = f"at {int(hour):02d}:{int(minute):02d}"
    else:
        time_clause = f"at minute(s) {minute} of hour(s) {hour}"

    day_clause = ""
    if dow != "*":
        day_clause = ", on " + _describe_dow(dow)
    elif dom != "*":
        day_clause = f", on day-of-month {dom}"

    month_clause = ""
    if month != "*":
        month_clause = ", in " + _describe_month(month)

    return f"{time_clause}{day_clause}{month_clause}".strip()


def _describe_dow(dow: str) -> str:
    if "-" in dow and "/" not in dow:
        a, b = dow.split("-")
        try:
            return f"{_DOW_LABELS[int(a) % 7]} through {_DOW_LABELS[int(b) % 7]}"
        except (ValueError, IndexError):
            return dow
    labels: List[str] = []
    for tok in dow.split(","):
        if tok.isdigit():
            labels.append(_DOW_LABELS[int(tok) % 7])
        else:
            return dow
    return ", ".join(labels)


def _describe_month(month: str) -> str:
    labels: List[str] = []
    for tok in month.split(","):
        if tok.isdigit() and 1 <= int(tok) <= 12:
            labels.append(_MONTH_LABELS[int(tok)])
        else:
            return month
    return ", ".join(labels)
