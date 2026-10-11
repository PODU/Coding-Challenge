import json
import re
import time

import pytest

from ccprox.cli import build_parser

TAG = "AA:00:00:00:00:01"
OTHER = "AA:00:00:00:00:02"


def test_survey_ranks_strongest_first_and_drops_stale(run_cli):
    events = [
        (0.0, OTHER, "Weak", -80),
        (0.0, TAG, "Strong", -50),
        (0.0, "AA:00:00:00:00:03", None, -65),
    ]
    code, out, _ = run_cli(["--interval", "0.2", "--duration", "0.5", "--timeout", "5"], events)
    assert code == 0
    last = out.strip().split("\n\n")[-1]
    names = [ln.split()[1] for ln in last.splitlines()[1:4]]
    assert names == ["Strong", "AA:00:00:00:00:03", "Weak"]
    assert "3 device(s)" in last


def test_survey_device_disappears_after_timeout(run_cli):
    events = [(0.0, TAG, "Blip", -60)]
    code, out, _ = run_cli(["--interval", "0.15", "--duration", "0.8", "--timeout", "0.3"], events)
    frames = out.strip().split("\n\n")
    assert "Blip" in frames[0]
    assert "Blip" not in frames[-1]
    assert "0 device(s)" in frames[-1]


def test_survey_verbose_reports_raw_vs_counted(run_cli):
    events = [(0.0, TAG, "Tag", -60)] + [(0.01, TAG, "Tag", -60)] * 4 + [(0.01, TAG, "Tag", -61)]
    code, out, _ = run_cli(["-v", "--interval", "0.3", "--duration", "0.35"], events)
    assert "readings raw 6, counted 2, discarded 4" in out


def test_hunt_filters_target_and_shows_band(run_cli):
    events = [(0.02, OTHER, "Other", -40), (0.02, TAG, "Tag", -50), (0.02, OTHER, "Other", -41),
              (0.02, TAG, "Tag", -52)]
    code, out, _ = run_cli(["--hunt", "tag", "--duration", "0.3"], events)
    assert code == 0
    lines = [ln for ln in out.splitlines() if ln.startswith("Tag ")]
    assert len(lines) == 2
    assert "NEAR" in lines[0] and "RSSI  -50 dBm" in lines[0]
    assert "avg  -51.0" in lines[1]
    assert "Other" not in out
    assert "~" in lines[0] and " m" in lines[0]  # distance estimate shown
    assert "not direction" in out  # disclaimer


def test_hunt_by_address(run_cli):
    code, out, _ = run_cli(["--hunt", TAG.lower(), "--duration", "0.2"], [(0.02, TAG, None, -60)])
    assert code == 0 and "Hunting AA:00:00:00:00:01" in out


def test_hunt_target_never_appears_is_an_error(run_cli):
    code, out, err = run_cli(["--hunt", "Ghost", "--start-timeout", "0.3"],
                             [(0.02, OTHER, "Other", -50)])
    assert code == 1
    assert "target 'Ghost' not found within 0.3s" in err


def test_hunt_flags_lost_then_reacquired(run_cli):
    events = [(0.02, TAG, "Tag", -50), (0.02, TAG, "Tag", -51), (0.7, TAG, "Tag", -55)]
    code, out, _ = run_cli(["--hunt", "Tag", "--timeout", "0.3", "--duration", "1.0"], events)
    assert code == 0
    lost = out.index("TARGET LOST: Tag")
    back = out.index("Target reacquired: Tag")
    assert lost < back
    assert "RSSI  -55 dBm" in out[back:]


def test_hunt_band_follows_overrides(run_cli):
    ev = [(0.02, TAG, "Tag", -60)]
    _, default_out, _ = run_cli(["--hunt", "Tag", "--duration", "0.2"], ev)
    _, near_out, _ = run_cli(["--hunt", "Tag", "--duration", "0.2", "--near", "-58"], ev)
    _, far_out, _ = run_cli(["--hunt", "Tag", "--duration", "0.2", "--far", "-58"], ev)
    assert re.search(r"Tag\s+MEDIUM", default_out)
    assert re.search(r"Tag\s+NEAR", near_out)
    assert re.search(r"Tag\s+FAR", far_out)


def test_hunt_verbose_counts_duplicates(run_cli):
    events = [(0.02, TAG, "Tag", -50)] + [(0.01, TAG, "Tag", -50)] * 3
    _, out, _ = run_cli(["--hunt", "Tag", "-v", "--duration", "0.3"], events)
    assert "raw 4 counted 1" in out


def test_hunt_sound_clicks_then_goes_silent_when_lost(run_cli):
    clicks = []
    events = [(0.02, TAG, "Tag", -44)] * 1 + [(0.05, TAG, "Tag", v) for v in (-45, -44, -46)]
    t0 = time.monotonic()
    code, out, _ = run_cli(["--hunt", "Tag", "--sound", "--timeout", "0.3", "--duration", "1.2"],
                           events, sink=lambda s: clicks.append(time.monotonic() - t0))
    assert code == 0 and "TARGET LOST" in out
    assert len(clicks) >= 3  # fast clicking while close
    # last reading at ~0.17 s + 0.3 s timeout: nothing should click after ~0.6 s
    assert max(clicks) < 0.65


def test_calibrate_saves_and_hunt_reuses_it(run_cli, tmp_path):
    events = [(0.02, TAG, "Tag", v) for v in (-48, -50, -49, -30, -50, -51)]
    code, out, _ = run_cli(["--calibrate", "Tag", "--yes", "--seconds", "0.4"], events)
    assert code == 0, out
    assert "near reference -49.5 dBm" in out
    saved = json.loads((tmp_path / "calibration.json").read_text())
    assert saved == {"tag": -49.5, TAG.lower(): -49.5}

    # hunting by address picks up the reference saved for that device
    code, out, _ = run_cli(["--hunt", TAG, "--duration", "0.2"], [(0.02, TAG, "Tag", -49)])
    assert "near -49.5 dBm" in out
    assert re.search(r"Tag\s+IMMEDIATE", out)
    # an explicit flag still wins
    _, out, _ = run_cli(["--hunt", "Tag", "--near", "-40", "--duration", "0.2"],
                        [(0.02, TAG, "Tag", -49)])
    assert "near -40 dBm" in out


def test_calibrate_too_few_samples(run_cli, tmp_path):
    code, _, err = run_cli(["--calibrate", "Tag", "--yes", "--seconds", "0.2"],
                           [(0.02, TAG, "Tag", -50)])
    assert code == 1 and "need at least 3" in err
    assert not (tmp_path / "calibration.json").exists()


def test_calibrate_not_found(run_cli):
    code, _, err = run_cli(["--calibrate", "Ghost", "--yes", "--start-timeout", "0.2"], [])
    assert code == 1 and "not found" in err


def test_log_csv(run_cli, tmp_path):
    log = tmp_path / "s.csv"
    run_cli(["--interval", "0.2", "--duration", "0.3", "--log", str(log)],
            [(0.01, TAG, "Tag", -50), (0.01, OTHER, None, -70)])
    rows = log.read_text().splitlines()
    assert rows[0] == "time,station,transport,address,name,rssi"
    assert rows[1].endswith(f"{TAG},Tag,-50") and rows[2].endswith(f"{OTHER},,-70")


@pytest.mark.parametrize("argv,msg", [
    (["--near", "-80"], "must be stronger"),
    (["--window", "0"], "--window"),
    (["--interval", "0"], "--interval"),
    (["--path-loss", "-1"], "--path-loss"),
    (["--hunt", "a", "--calibrate", "b"], "not allowed with"),
])
def test_usage_errors(run_cli, capsys, argv, msg):
    with pytest.raises(SystemExit) as e:
        run_cli(argv, [])
    assert e.value.code == 2
    assert msg in capsys.readouterr().err


def test_help_documents_distance_not_direction():
    text = build_parser().format_help()
    assert "HOW FAR, never WHICH WAY" in text
    assert "--path-loss" in text and "--near" in text and "--far" in text and "-72" in text


def test_simulator_end_to_end(run_cli):
    from ccprox.scanner import SimSource

    code, out, _ = run_cli(["--hunt", "SimTag", "--duration", "2.5", "-v"],
                           source_factory=lambda q, clock: SimSource(q, clock=clock))
    assert code == 0 and "Hunting SimTag" in out
    raw, counted = map(int, re.findall(r"raw (\d+) counted (\d+)", out)[-1])
    assert raw > counted > 0  # simulator injects cached duplicates
