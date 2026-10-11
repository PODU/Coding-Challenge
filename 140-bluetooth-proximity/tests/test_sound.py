import asyncio

from ccprox.proximity import Calibration
from ccprox.sound import Clicker


class Clock:
    def __init__(self):
        self.t = 0.0

    def __call__(self):
        return self.t


def test_silent_until_first_reading_and_after_timeout():
    clk = Clock()
    c = Clicker(Calibration(), timeout_s=10, sink=lambda s: None, clock=clk)
    assert c.next_interval() is None
    c.update(-50)
    assert c.next_interval() is not None
    clk.t = 9.9
    assert c.next_interval() is not None
    clk.t = 10.1
    assert c.next_interval() is None  # target lost -> silence


def test_tempo_tracks_smoothed_signal():
    c = Clicker(Calibration(), timeout_s=10, sink=lambda s: None, clock=Clock())
    c.update(-70)
    slow = c.next_interval()
    c.update(-46)
    assert c.next_interval() < slow


def test_clicks_more_often_when_closer():
    async def count(rssi):
        hits = []
        c = Clicker(Calibration(), timeout_s=10, sink=hits.append)
        c.update(rssi)
        c.start()
        await asyncio.sleep(0.8)
        await c.stop()
        return hits

    near = asyncio.run(count(-44))
    far = asyncio.run(count(-80))
    assert len(near) > 3 * len(far)
    assert near[0] == 1.0 and far[0] == 0.0  # strength (pitch) handed to the sink
