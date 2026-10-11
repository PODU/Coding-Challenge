import pytest

from ccprox.tracker import Reading, Tracker, matches


def r(rssi, t, addr="AA", name="tag"):
    return Reading(addr, name, rssi, t)


def test_cached_duplicate_within_window_is_discarded():
    tr = Tracker(dedupe_s=0.2)
    assert tr.ingest(r(-60, 0.0)) is not None
    assert tr.ingest(r(-60, 0.1)) is None  # same value, 100 ms later -> cached repeat
    st = tr.devices["AA"]
    assert (st.raw, st.counted) == (2, 1)
    assert list(st.window) == [-60]


def test_same_value_after_window_counts():
    tr = Tracker(dedupe_s=0.2)
    tr.ingest(r(-60, 0.0))
    assert tr.ingest(r(-60, 0.25)) is not None


def test_different_value_counts_even_when_fast():
    tr = Tracker(dedupe_s=0.2)
    tr.ingest(r(-60, 0.0))
    assert tr.ingest(r(-61, 0.01)) is not None


def test_stream_of_cached_repeats_stays_discarded():
    tr = Tracker(dedupe_s=0.2)
    tr.ingest(r(-60, 0.0))
    for i in range(1, 10):
        assert tr.ingest(r(-60, i * 0.15)) is None


def test_rolling_window_moving_average():
    tr = Tracker(window=3)
    for i, v in enumerate([-50, -60, -70, -80]):
        tr.ingest(r(v, i))
    st = tr.devices["AA"]
    assert list(st.window) == [-60, -70, -80]
    assert st.smoothed == pytest.approx(-70)
    assert st.latest == -80


def test_ranked_strongest_first_and_expiry():
    tr = Tracker(timeout_s=10)
    tr.ingest(r(-80, 0, "A", "far"))
    tr.ingest(r(-40, 0, "B", "near"))
    tr.ingest(r(-60, 5, "C", "mid"))
    assert [d.name for d in tr.ranked(5)] == ["near", "mid", "far"]
    # at t=12, A and B have been silent for > 10 s
    assert [d.name for d in tr.ranked(12)] == ["mid"]
    gone = tr.expire(12)
    assert sorted(d.name for d in gone) == ["far", "near"]
    assert list(tr.devices) == ["C"]


def test_name_learned_later_and_label_falls_back_to_address():
    tr = Tracker()
    tr.ingest(Reading("AA:BB", None, -50, 0))
    assert tr.devices["AA:BB"].label == "AA:BB"
    tr.ingest(Reading("aa:bb", "Phone", -51, 1))  # address is case-insensitive
    assert tr.devices["AA:BB"].label == "Phone"
    tr.ingest(Reading("AA:BB", None, -52, 2))  # a nameless advert doesn't erase it
    assert tr.devices["AA:BB"].label == "Phone"


def test_totals():
    tr = Tracker()
    tr.ingest(r(-50, 0.0))
    tr.ingest(r(-50, 0.05))
    tr.ingest(r(-51, 0.1))
    assert tr.totals() == (3, 2)


def test_matches():
    assert matches("Phone", "AA:BB", "phone")
    assert matches("aa:bb", "AA:BB", None)
    assert not matches("Phon", "AA:BB", "Phone")
    assert not matches("x", "AA:BB", None)


def test_bad_window():
    with pytest.raises(ValueError):
        Tracker(window=0)
