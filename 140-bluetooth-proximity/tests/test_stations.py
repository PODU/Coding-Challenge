import math
import socket
import threading
import time

import pytest

from ccprox import render
from ccprox.stations import Fix, decode, describe, encode, locate, parse_hostport
from ccprox.tracker import Reading

from .conftest import ScriptedSource

TAG = "AA:00:00:00:00:01"


def test_parse_hostport():
    assert parse_hostport("10.0.0.2:7140") == ("10.0.0.2", 7140)
    assert parse_hostport("7140") == ("0.0.0.0", 7140)
    assert parse_hostport(":7140", "127.0.0.1") == ("127.0.0.1", 7140)
    assert parse_hostport("[::1]:7140") == ("::1", 7140)
    for bad in ("host:http", "host:70000"):
        with pytest.raises(ValueError):
            parse_hostport(bad)


def test_wire_roundtrip():
    r = Reading(TAG, "Tag", -61, 1.0, transport="classic")
    back = decode(encode(r), "B", 5.0)
    assert back == Reading(TAG, "Tag", -61, 5.0, transport="classic", station="B")
    assert decode(b'{"ccprox": 1, "station": "x"}\n', "B", 0) is None  # hello
    assert decode(b"garbage\n", "B", 0) is None
    assert decode(b'{"rssi": "x", "address": "a"}', "B", 0) is None


def test_locate_exact_intersection():
    # A=(0,0), B=(4,0), target=(0,3): a 3-4-5 triangle
    f = locate(3, 5, 4)
    assert f.consistent and f.x == pytest.approx(0) and f.y == pytest.approx(3)
    f = locate(math.hypot(1, 2), math.hypot(3, 2), 4)  # target (1, ±2)
    assert f.x == pytest.approx(1) and f.y == pytest.approx(2)


@pytest.mark.parametrize("d_a,d_b,L,expect", [
    (1, 1, 4, 2.0),       # circles too small to meet: midway between them
    (1, 6, 3, -2.0),      # B's circle swallows A's: beyond A
    (5, 1, 3, 4.5),       # A's circle swallows B's: beyond B
    (16, 30.1, 1, -22.55),  # real noisy reading: stays sane (closed form gave -324 m)
])
def test_locate_best_fit_when_circles_miss(d_a, d_b, L, expect):
    f = locate(d_a, d_b, L)
    assert not f.consistent and f.y == 0
    assert f.x == pytest.approx(expect)


def test_locate_bad_baseline():
    with pytest.raises(ValueError):
        locate(1, 1, 0)


def test_describe():
    assert describe(Fix(-1, 0, False), 4).startswith("beyond A")
    assert describe(Fix(5, 0, False), 4).startswith("beyond B")
    assert describe(Fix(1, 0, True), 4) == "between A and B, nearer A"
    assert describe(Fix(3, 0, True), 4) == "between A and B, nearer B"
    assert describe(Fix(2.1, 0, True), 4) == "between A and B, about midway"


def test_strip_marks_position():
    s = render.strip(Fix(2, 1, True), 4)
    a, b, dot = s.index("A"), s.index("B"), s.index("●")
    assert a < dot < b and abs((dot - a) - (b - dot)) <= 1
    assert render.strip(Fix(-100, 0, False), 4).startswith("●")
    assert render.strip(Fix(100, 0, False), 4).endswith("●")


def free_port():
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def start_station(port, events, duration):
    """Run `ccprox --serve` in a thread with a scripted radio; wait until it listens."""
    import io

    from ccprox.cli import main

    out = io.StringIO()
    th = threading.Thread(target=main, kwargs=dict(
        argv=["--serve", f"127.0.0.1:{port}", "--station-name", "kitchen",
              "--duration", str(duration)],
        out=out, err=io.StringIO(),
        source_factory=lambda q, c: ScriptedSource(q, c, events)), daemon=True)
    th.start()
    deadline = time.monotonic() + 5
    while time.monotonic() < deadline:
        if "listening" in out.getvalue():
            return th, out
        time.sleep(0.02)
    raise AssertionError("station did not start")


def test_two_station_hunt(run_cli):
    port = free_port()
    # B hears the tag a little weaker than A does, all the way through
    b_events = [(0.05, TAG, "Tag", -60)] + [(0.05, TAG, "Tag", v) for v in [-61, -60, -59] * 20]
    th, station_out = start_station(port, b_events, 3.5)
    a_events = [(0.05, TAG, "Tag", v) for v in [-55, -56, -54] * 12]
    code, out, err = run_cli(["--hunt", "Tag", "--remote", f"127.0.0.1:{port}", "--baseline", "3",
                              "--duration", "1.5"], a_events)
    th.join(5)
    assert code == 0, err
    assert "Connected to station B 'kitchen'" in out
    lines = [ln for ln in out.splitlines() if ln.startswith("[")]
    assert lines, out
    last = lines[-1]
    # A ≈ -55 → 1.9 m, B ≈ -60 → 3.4 m, 3 m apart: inside the triangle, nearer A
    assert "nearer A" in last and "side unknown" in last
    assert re.search(r"A -5[456] dBm", last) and re.search(r"B -(59|60|61) dBm", last)
    assert "client connected" in station_out.getvalue()


def test_remote_station_disconnect_is_reported(run_cli):
    port = free_port()
    th, _ = start_station(port, [(0.05, TAG, "Tag", -60)] * 5, 0.6)
    a_events = [(0.1, TAG, "Tag", v) for v in [-55, -56] * 12]
    code, out, _ = run_cli(["--hunt", "Tag", "--remote", f"127.0.0.1:{port}", "--baseline", "3",
                            "--duration", "2"], a_events)
    th.join(5)
    assert code == 0
    assert "station disconnected" in out.splitlines()[-1]


def test_remote_unreachable(run_cli):
    port = free_port()  # nothing listening
    code, _, err = run_cli(["--hunt", "Tag", "--remote", f"127.0.0.1:{port}", "--baseline", "3"],
                           [])
    assert code == 5
    assert err.startswith("ccprox: cannot reach station") and "Traceback" not in err


def test_station_port_in_use(run_cli):
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        s.listen()
        port = s.getsockname()[1]
        code, _, err = run_cli(["--serve", f"127.0.0.1:{port}", "--duration", "0.2"], [])
    assert code == 5 and "cannot listen" in err


@pytest.mark.parametrize("argv,msg", [
    (["--hunt", "x", "--remote", "h:1"], "--baseline"),
    (["--hunt", "x", "--remote", "h:1", "--baseline", "0"], "--baseline"),
    (["--remote", "h:1", "--baseline", "2"], "only works with --hunt"),
    (["--hunt", "x", "--baseline", "2"], "need --remote"),
    (["--serve", "h:x"], "bad port"),
])
def test_station_usage_errors(run_cli, capsys, argv, msg):
    with pytest.raises(SystemExit) as e:
        run_cli(argv, [])
    assert e.value.code == 2 and msg in capsys.readouterr().err


import re  # noqa: E402
