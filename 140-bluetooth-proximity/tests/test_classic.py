import asyncio
import re

import pytest

from ccprox import classic
from ccprox.classic import (P_ADDR, P_PRESENT, P_RSSI, BluezClassicSource, MultiSource,
                            WinEndpoint, bluez_reading, classic_source)
from ccprox.scanner import AdapterError

dbus_fast = pytest.importorskip("dbus_fast")
from dbus_fast import Message, MessageType, Variant  # noqa: E402


def test_win_cached_strength_for_absent_device_is_ignored():
    ep = WinEndpoint("Crusher")
    # Added: Windows hands back the last-known strength, device not present
    assert ep.apply({P_ADDR: "88:08:94:64:8b:aa", P_RSSI: -63, P_PRESENT: False}) is None
    assert ep.address == "88:08:94:64:8B:AA"


def test_win_present_then_strength_update_reports():
    ep = WinEndpoint("cp60")
    assert ep.apply({P_ADDR: "41:42:ed:ef:8e:b5", P_RSSI: None, P_PRESENT: True}) is None
    assert ep.apply({P_RSSI: -61}) == -61  # Updated event with a fresh inquiry result


def test_win_unknown_and_out_of_range_strength_ignored():
    ep = WinEndpoint("x")
    ep.apply({P_ADDR: "aa", P_PRESENT: True})
    assert ep.apply({P_RSSI: -128}) is None  # Windows' "no measurement"
    assert ep.apply({P_RSSI: 50}) is None
    assert ep.apply({P_RSSI: -70}) == -70


def test_win_needs_an_address():
    assert WinEndpoint("x").apply({P_RSSI: -50, P_PRESENT: True}) is None


def test_bluez_reading():
    r = bluez_reading({"Address": "00:11:22:33:44:55", "Name": "Speaker", "RSSI": -58})
    assert (r.address, r.name, r.rssi, r.transport) == ("00:11:22:33:44:55", "Speaker", -58, "classic")
    # BlueZ's Alias defaults to the address with dashes; that's not a name
    r = bluez_reading({"Address": "00:11:22:33:44:55", "Alias": "00-11-22-33-44-55", "RSSI": -70})
    assert r.name is None
    assert bluez_reading({"Address": "00:11:22:33:44:55", "Name": "x"}) is None


def reply(body=(), error=None):
    if error:
        return Message.new_error(Message(serial=1, path="/", member="x"), error, "nope")
    return Message(message_type=MessageType.METHOD_RETURN, reply_serial=1, body=list(body),
                   signature="a{oa{sa{sv}}}" if body else "")


ADAPTER = "/org/bluez/hci0"
DEV = ADAPTER + "/dev_00_11_22_33_44_55"


class FakeBus:
    objs = {}
    fail_on = None
    instances = []

    def __init__(self, bus_type=None):
        self.calls = []
        self.handlers = []
        self.disconnected = False
        FakeBus.instances.append(self)

    async def connect(self):
        return self

    async def call(self, msg):
        self.calls.append((msg.member, msg.body))
        if msg.member == self.fail_on:
            return reply(error="org.bluez.Error.NotReady")
        if msg.member == "GetManagedObjects":
            return reply([self.objs])
        return reply()

    def add_message_handler(self, h):
        self.handlers.append(h)

    def disconnect(self):
        self.disconnected = True


@pytest.fixture
def fakebus():
    FakeBus.instances = []
    FakeBus.fail_on = None
    FakeBus.objs = {ADAPTER: {"org.bluez.Adapter1": {"Powered": Variant("b", True)}}}
    return FakeBus


def run(coro):
    return asyncio.run(coro)


def test_bluez_starts_bredr_discovery_and_reports_rssi(fakebus):
    async def go():
        q = asyncio.Queue()
        src = BluezClassicSource(q, bus_factory=FakeBus)
        await src.start()
        bus = fakebus.instances[0]
        members = [m for m, _ in bus.calls]
        assert members[-2:] == ["SetDiscoveryFilter", "StartDiscovery"]
        filt = dict(bus.calls)["SetDiscoveryFilter"][0]
        assert filt["Transport"].value == "bredr"
        # a device appears, then its RSSI changes
        h = bus.handlers[0]
        h(Message(path="/", interface="org.freedesktop.DBus.ObjectManager",
                  member="InterfacesAdded", message_type=MessageType.SIGNAL, signature="oa{sa{sv}}",
                  body=[DEV, {"org.bluez.Device1": {"Address": Variant("s", "00:11:22:33:44:55"),
                                                    "Name": Variant("s", "Speaker"),
                                                    "RSSI": Variant("n", -66)}}]))
        h(Message(path=DEV, interface="org.freedesktop.DBus.Properties", member="PropertiesChanged",
                  message_type=MessageType.SIGNAL, signature="sa{sv}as",
                  body=["org.bluez.Device1", {"RSSI": Variant("n", -52)}, []]))
        # unrelated property change: no reading
        h(Message(path=DEV, interface="org.freedesktop.DBus.Properties", member="PropertiesChanged",
                  message_type=MessageType.SIGNAL, signature="sa{sv}as",
                  body=["org.bluez.Device1", {"Connected": Variant("b", False)}, []]))
        got = [q.get_nowait() for _ in range(q.qsize())]
        assert [(r.name, r.rssi, r.transport) for r in got] == [("Speaker", -66, "classic"),
                                                               ("Speaker", -52, "classic")]
        await src.stop()
        assert bus.calls[-1][0] == "StopDiscovery" and bus.disconnected

    run(go())


def test_bluez_no_adapter(fakebus):
    fakebus.objs = {}
    with pytest.raises(AdapterError, match="No Bluetooth adapter"):
        run(BluezClassicSource(asyncio.Queue(), bus_factory=FakeBus).start())


def test_bluez_powered_off(fakebus):
    fakebus.objs = {ADAPTER: {"org.bluez.Adapter1": {"Powered": Variant("b", False)}}}
    with pytest.raises(AdapterError, match="powered off"):
        run(BluezClassicSource(asyncio.Queue(), bus_factory=FakeBus).start())


def test_bluez_dbus_error_translated(fakebus):
    fakebus.fail_on = "StartDiscovery"
    with pytest.raises(AdapterError, match="powered off"):  # org.bluez.Error.NotReady
        run(BluezClassicSource(asyncio.Queue(), bus_factory=FakeBus).start())


def test_classic_unsupported_on_macos(monkeypatch):
    monkeypatch.setattr(classic.sys, "platform", "darwin")
    with pytest.raises(AdapterError) as e:
        classic_source(asyncio.Queue())
    assert e.value.exit_code == 2 and "--transport le" in str(e.value)


def test_classic_source_per_platform(monkeypatch):
    monkeypatch.setattr(classic.sys, "platform", "linux")
    assert isinstance(classic_source(asyncio.Queue()), BluezClassicSource)
    monkeypatch.setattr(classic.sys, "platform", "win32")
    assert isinstance(classic_source(asyncio.Queue()), classic.WinClassicSource)


def test_multisource_stops_started_sources_when_one_fails():
    log = []

    class Ok:
        async def start(self): log.append("ok start")
        async def stop(self): log.append("ok stop")

    class Bad:
        async def start(self): raise AdapterError("boom")
        async def stop(self): log.append("bad stop")

    with pytest.raises(AdapterError):
        run(MultiSource(Ok(), Bad()).start())
    assert log == ["ok start", "ok stop"]


def test_survey_both_transports_shows_type_column(run_cli):
    from ccprox.scanner import SimSource

    code, out, _ = run_cli(["--transport", "both", "--interval", "2.2", "--duration", "2.3"],
                           source_factory=lambda q, c: SimSource(q, clock=c,
                                                                  transports=("le", "classic")))
    last = out.strip().split("\n\n")[-1]
    assert "TYPE" in last.splitlines()[0]
    assert re.search(r"SimSpeaker\s+AA:BB:CC:00:00:06 BR", last)
    assert re.search(r"SimTag\s+AA:BB:CC:00:00:01 LE", last)


def test_same_address_over_le_and_classic_tracked_separately():
    from ccprox.tracker import Reading, Tracker

    tr = Tracker()
    tr.ingest(Reading("AA:00", "Dual", -50, 0, transport="le"))
    tr.ingest(Reading("AA:00", "Dual", -70, 0, transport="classic"))
    assert sorted(tr.devices) == ["AA:00", "AA:00/classic"]
    assert tr.devices["AA:00/classic"].smoothed == -70
