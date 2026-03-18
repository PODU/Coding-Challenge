"""Cron parsing + next-fire computation against an injected clock."""

from __future__ import annotations

from datetime import datetime, timezone

import pytest

from ai_agent_scheduling.cron.expression import CronError, CronExpression, describe


def dt(y, mo, d, h, mi):
    return datetime(y, mo, d, h, mi, tzinfo=timezone.utc)


def test_parse_basic_fields():
    c = CronExpression.parse("0 9 * * 1-5")
    assert c.minutes == {0}
    assert c.hours == {9}
    assert c.dows == {1, 2, 3, 4, 5}
    assert c.dom_restricted is False
    assert c.dow_restricted is True


@pytest.mark.parametrize(
    "expr,ok",
    [
        ("* * * * *", True),
        ("*/5 * * * *", True),
        ("0 9 * * 1-5", True),
        ("0 0,12 1 */2 *", True),
        ("0 9 * * 7", True),       # 7 == Sunday
        ("60 * * * *", False),     # minute out of range
        ("0 24 * * *", False),     # hour out of range
        ("0 9 * * 8", False),      # dow out of range
        ("0 9 * *", False),        # too few fields
        ("0 9 * * * *", False),    # too many fields
        ("a b c d e", False),      # garbage
    ],
)
def test_validation(expr, ok):
    assert CronExpression.is_valid(expr) is ok


def test_next_fire_weekday_9am_from_friday_evening_skips_weekend():
    c = CronExpression.parse("0 9 * * 1-5")
    friday_evening = dt(2025, 6, 6, 18, 0)  # Fri 2025-06-06
    nxt = c.next_fire(friday_evening)
    assert nxt == dt(2025, 6, 9, 9, 0)  # Monday 09:00


def test_next_fire_every_5_minutes():
    c = CronExpression.parse("*/5 * * * *")
    assert c.next_fire(dt(2025, 6, 2, 9, 2)) == dt(2025, 6, 2, 9, 5)
    assert c.next_fire(dt(2025, 6, 2, 9, 5)) == dt(2025, 6, 2, 9, 10)


def test_next_fire_is_strictly_after():
    c = CronExpression.parse("0 9 * * *")
    # exactly on a fire time -> returns the *next* day's fire
    assert c.next_fire(dt(2025, 6, 2, 9, 0)) == dt(2025, 6, 3, 9, 0)


def test_next_fire_crosses_month_and_year():
    c = CronExpression.parse("30 23 31 12 *")  # 23:30 on Dec 31
    assert c.next_fire(dt(2025, 1, 1, 0, 0)) == dt(2025, 12, 31, 23, 30)
    assert c.next_fire(dt(2025, 12, 31, 23, 30)) == dt(2026, 12, 31, 23, 30)


def test_dom_or_dow_semantics():
    # Both restricted -> match on EITHER day-of-month 1 OR Sunday(0).
    c = CronExpression.parse("0 0 1 * 0")
    # 2025-06-01 is itself a Sunday and the 1st.
    assert c.matches(dt(2025, 6, 1, 0, 0))
    # 2025-06-08 is a Sunday but not the 1st -> still matches via dow.
    assert c.matches(dt(2025, 6, 8, 0, 0))
    # 2025-07-01 is the 1st but a Tuesday -> matches via dom.
    assert c.matches(dt(2025, 7, 1, 0, 0))
    # 2025-06-03 is neither -> no match.
    assert not c.matches(dt(2025, 6, 3, 0, 0))


def test_describe_is_human_readable():
    assert "09:00" in describe("0 9 * * 1-5")
    assert "Monday through Friday" in describe("0 9 * * 1-5")
    assert "every minute" in describe("* * * * *")


def test_parse_raises_on_bad_expression():
    with pytest.raises(CronError):
        CronExpression.parse("nope")
