"""Unit tests for the pure interval arithmetic."""

from datetime import UTC, datetime

import pytest

from scheduler.domain.interval import Interval, intersect, merge, subtract


def u(h, m=0):
    """Build a 2026-07-01 UTC datetime at the given hour/minute."""
    return datetime(2026, 7, 1, h, m, tzinfo=UTC)


class TestIntervalConstruction:
    def test_rejects_naive_start(self):
        with pytest.raises(ValueError):
            Interval(datetime(2026, 7, 1, 9), u(10))

    def test_rejects_end_before_start(self):
        with pytest.raises(ValueError):
            Interval(u(10), u(9))

    def test_rejects_zero_length(self):
        with pytest.raises(ValueError):
            Interval(u(10), u(10))

    def test_normalises_to_utc(self):
        from zoneinfo import ZoneInfo

        local = datetime(2026, 7, 1, 11, tzinfo=ZoneInfo("Europe/London"))
        iv = Interval(local, local.replace(hour=12))
        assert iv.start.tzinfo == UTC
        assert iv.start.hour == 10  # BST is UTC+1

    def test_duration(self):
        assert Interval(u(9), u(10, 30)).duration.total_seconds() == 5400


class TestOverlap:
    def test_abutting_do_not_overlap(self):
        assert not Interval(u(9), u(10)).overlaps(Interval(u(10), u(11)))

    def test_overlapping(self):
        assert Interval(u(9), u(11)).overlaps(Interval(u(10), u(12)))

    def test_clip_no_overlap_returns_none(self):
        assert Interval(u(9), u(10)).clip(Interval(u(11), u(12))) is None

    def test_clip(self):
        clipped = Interval(u(9), u(12)).clip(Interval(u(10), u(11)))
        assert clipped == Interval(u(10), u(11))


class TestMerge:
    def test_empty(self):
        assert merge([]) == []

    def test_single(self):
        iv = Interval(u(9), u(10))
        assert merge([iv]) == [iv]

    def test_disjoint_sorted(self):
        a, b = Interval(u(11), u(12)), Interval(u(9), u(10))
        assert merge([a, b]) == [b, a]

    def test_overlapping_merged(self):
        result = merge([Interval(u(9), u(11)), Interval(u(10), u(12))])
        assert result == [Interval(u(9), u(12))]

    def test_adjacent_coalesced(self):
        result = merge([Interval(u(9), u(10)), Interval(u(10), u(11))])
        assert result == [Interval(u(9), u(11))]

    def test_nested_absorbed(self):
        result = merge([Interval(u(9), u(13)), Interval(u(10), u(11))])
        assert result == [Interval(u(9), u(13))]


class TestSubtract:
    def test_no_busy(self):
        base = Interval(u(9), u(17))
        assert subtract(base, []) == [base]

    def test_busy_inside_splits(self):
        result = subtract(Interval(u(9), u(17)), [Interval(u(12), u(13))])
        assert result == [Interval(u(9), u(12)), Interval(u(13), u(17))]

    def test_busy_spanning_removes_all(self):
        assert subtract(Interval(u(9), u(17)), [Interval(u(8), u(18))]) == []

    def test_busy_abutting_keeps_base(self):
        base = Interval(u(9), u(17))
        assert subtract(base, [Interval(u(17), u(18))]) == [base]

    def test_busy_outside_keeps_base(self):
        base = Interval(u(9), u(17))
        assert subtract(base, [Interval(u(20), u(21))]) == [base]

    def test_multiple_busy(self):
        result = subtract(
            Interval(u(9), u(17)),
            [Interval(u(10), u(11)), Interval(u(13), u(14))],
        )
        assert result == [
            Interval(u(9), u(10)),
            Interval(u(11), u(13)),
            Interval(u(14), u(17)),
        ]

    def test_busy_at_edges(self):
        result = subtract(Interval(u(9), u(17)), [Interval(u(9), u(10)), Interval(u(16), u(17))])
        assert result == [Interval(u(10), u(16))]


class TestIntersect:
    def test_intersection(self):
        a = [Interval(u(9), u(12))]
        b = [Interval(u(10), u(14))]
        assert intersect(a, b) == [Interval(u(10), u(12))]

    def test_no_intersection(self):
        assert intersect([Interval(u(9), u(10))], [Interval(u(11), u(12))]) == []
