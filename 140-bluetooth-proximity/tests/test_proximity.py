import pytest

from ccprox.proximity import (MAX_CLICK_S, MIN_CLICK_S, Calibration, click_interval,
                              estimate_distance, load_calibrations, reference_from_samples,
                              save_calibration)


def test_default_bands():
    c = Calibration()  # near -45, far -72, midpoint -58.5
    assert c.band(-40) == "IMMEDIATE"
    assert c.band(-45) == "IMMEDIATE"
    assert c.band(-50) == "NEAR"
    assert c.band(-58.5) == "NEAR"
    assert c.band(-65) == "MEDIUM"
    assert c.band(-72) == "MEDIUM"
    assert c.band(-80) == "FAR"


def test_overrides_shift_bands():
    assert Calibration().band(-50) == "NEAR"
    assert Calibration(near=-55).band(-50) == "IMMEDIATE"
    assert Calibration(near=-45, far=-60).band(-65) == "FAR"
    assert Calibration(near=-45, far=-90).band(-65) == "NEAR"


def test_near_must_be_stronger_than_far():
    with pytest.raises(ValueError):
        Calibration(near=-80, far=-72)
    with pytest.raises(ValueError):
        Calibration(near=-72, far=-72)


def test_fraction_clamped():
    c = Calibration()
    assert c.fraction(-30) == 1.0
    assert c.fraction(-100) == 0.0
    assert c.fraction(c.mid) == pytest.approx(0.5)


def test_distance_model():
    # at the reference point the estimate is the reference distance
    assert estimate_distance(-45, -45, 0.6, 2) == pytest.approx(0.6)
    # n=2: every 20 dB of extra loss is 10x the distance
    assert estimate_distance(-65, -45, 1.0, 2) == pytest.approx(10.0)
    # n=3: 30 dB per decade
    assert estimate_distance(-75, -45, 1.0, 3) == pytest.approx(10.0)
    # stronger than the reference -> closer than the reference
    assert estimate_distance(-39, -45, 1.0, 2) == pytest.approx(10 ** -0.3)
    with pytest.raises(ValueError):
        estimate_distance(-50, -45, 1, 0)


def test_click_interval_faster_when_stronger():
    c = Calibration()
    seq = [click_interval(v, c) for v in (-90, -72, -65, -58, -50, -45, -30)]
    assert seq == sorted(seq, reverse=True)
    assert seq[0] == pytest.approx(MAX_CLICK_S)
    assert seq[-1] == pytest.approx(MIN_CLICK_S)


def test_reference_median_resists_spikes():
    assert reference_from_samples([-50, -49, -51, -20, -50]) == -50
    with pytest.raises(ValueError):
        reference_from_samples([])


def test_calibration_store_roundtrip(tmp_path):
    p = tmp_path / "sub" / "cal.json"
    assert load_calibrations(p) == {}
    save_calibration("AA:BB", -48.5, p)
    save_calibration("Phone", -47, p)
    assert load_calibrations(p) == {"aa:bb": -48.5, "phone": -47.0}


def test_corrupt_store_ignored(tmp_path):
    p = tmp_path / "cal.json"
    p.write_text("{not json")
    assert load_calibrations(p) == {}
