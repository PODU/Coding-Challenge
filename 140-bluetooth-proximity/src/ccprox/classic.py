"""Classic Bluetooth (BR/EDR) as a second signal source.

bleak only speaks LE, so classic inquiry goes straight to the OS:

* Windows — WinRT ``DeviceWatcher`` over Bluetooth association endpoints, which
  reports ``System.Devices.Aep.SignalStrength`` from inquiry. A watcher reports
  each device once, so it is restarted every ``cycle_s`` seconds.
* Linux — BlueZ over D-Bus (``dbus-fast``, already a bleak dependency there):
  discovery with ``Transport=bredr`` and ``RSSI`` from ``PropertiesChanged``.
* macOS — not supported: inquiry is only exposed through IOBluetooth.

Only devices that are discoverable (or already paired, on Windows) answer an
inquiry, and inquiry RSSI updates far less often than LE advertisements.
"""

from __future__ import annotations

import asyncio
import sys
import time

from .scanner import AdapterError, NO_ADAPTER_MSG, translate
from .tracker import Reading

# Windows

BT_CLASSIC_PROTOCOL = "{e0cbf06c-cd8b-4647-bb8a-263b43f0f974}"
AQS = f'System.Devices.Aep.ProtocolId:="{BT_CLASSIC_PROTOCOL}"'
P_ADDR = "System.Devices.Aep.DeviceAddress"
P_RSSI = "System.Devices.Aep.SignalStrength"
P_PRESENT = "System.Devices.Aep.IsPresent"
UNKNOWN_RSSI = -128  # what Windows reports when it has no measurement


class WinEndpoint:
    """What we know about one association endpoint across watcher events."""

    def __init__(self, name: str | None = None) -> None:
        self.name = name
        self.address: str | None = None
        self.present = False

    def apply(self, props: dict) -> int | None:
        """Merge an Added/Updated property set. Returns an RSSI to report, if any.

        Windows hands back its cached last-known strength for devices that are
        not currently present; only a strength for a present device counts."""
        if props.get(P_ADDR):
            self.address = str(props[P_ADDR]).upper()
        if props.get(P_PRESENT) is not None:
            self.present = bool(props[P_PRESENT])
        rssi = props.get(P_RSSI)
        if rssi is None or self.address is None or not self.present:
            return None
        rssi = int(rssi)
        if rssi == UNKNOWN_RSSI or not -127 <= rssi <= 20:
            return None
        return rssi


def _unbox(v):
    import winrt.system as ws

    for f in (ws.unbox_string, ws.unbox_int32, ws.unbox_int16, ws.unbox_int8, ws.unbox_boolean):
        try:
            return f(v)
        except Exception:
            pass
    return None


def _props(winprops) -> dict:
    out = {}
    for k in (P_ADDR, P_RSSI, P_PRESENT):
        try:
            if winprops.has_key(k):
                v = winprops.lookup(k)
                out[k] = None if v is None else _unbox(v)
        except Exception:
            pass
    return out


class WinClassicSource:
    def __init__(self, queue: asyncio.Queue, clock=time.monotonic, cycle_s: float = 8.0) -> None:
        self.queue, self.clock, self.cycle_s = queue, clock, cycle_s
        self.endpoints: dict[str, WinEndpoint] = {}
        self._task: asyncio.Task | None = None
        self._watcher = None

    def _emit(self, ep: WinEndpoint, rssi: int | None) -> None:
        if rssi is not None:
            r = Reading(ep.address, ep.name, rssi, self.clock(), transport="classic")
            self._loop.call_soon_threadsafe(self.queue.put_nowait, r)

    def _added(self, _w, info) -> None:
        ep = self.endpoints.setdefault(info.id, WinEndpoint())
        ep.name = info.name or ep.name
        self._emit(ep, ep.apply(_props(info.properties)))

    def _updated(self, _w, upd) -> None:
        ep = self.endpoints.get(upd.id)
        if ep is not None:
            self._emit(ep, ep.apply(_props(upd.properties)))

    def _new_watcher(self):
        from winrt.windows.devices.enumeration import DeviceInformation, DeviceInformationKind

        w = DeviceInformation.create_watcher(AQS, [P_ADDR, P_RSSI, P_PRESENT],
                                             DeviceInformationKind.ASSOCIATION_ENDPOINT)
        w.add_added(self._added)
        w.add_updated(self._updated)
        return w

    async def _check_radio(self) -> None:
        from winrt.windows.devices.radios import Radio, RadioKind, RadioState

        radios = [r for r in await Radio.get_radios_async() if r.kind == RadioKind.BLUETOOTH]
        if not radios:
            raise AdapterError(NO_ADAPTER_MSG.replace("LE ", ""))
        if all(r.state != RadioState.ON for r in radios):
            raise translate(RuntimeError("radio is off"))

    async def _run(self) -> None:
        while True:
            self._watcher = self._new_watcher()
            self._watcher.start()
            await asyncio.sleep(self.cycle_s)
            self._watcher.stop()
            for ep in self.endpoints.values():
                ep.present = False  # must be re-confirmed by the next inquiry

    async def start(self) -> None:
        self._loop = asyncio.get_running_loop()
        try:
            await self._check_radio()
            self._new_watcher()  # fail fast on API errors before going async
        except AdapterError:
            raise
        except Exception as e:
            raise translate(e) from e
        self._task = asyncio.create_task(self._run())

    async def stop(self) -> None:
        if self._task:
            self._task.cancel()
            try:
                await self._task
            except asyncio.CancelledError:
                pass
        if self._watcher is not None:
            try:
                self._watcher.stop()
            except Exception:
                pass


# Linux (BlueZ)

DEVICE_IFACE = "org.bluez.Device1"
ADAPTER_IFACE = "org.bluez.Adapter1"


def unwrap(d: dict) -> dict:
    """dbus-fast Variants -> plain values."""
    return {k: getattr(v, "value", v) for k, v in d.items()}


def bluez_reading(props: dict, clock=time.monotonic) -> Reading | None:
    """A Reading from a (plain) org.bluez.Device1 property dict, if it has RSSI."""
    if props.get("RSSI") is None or not props.get("Address"):
        return None
    name = props.get("Name") or props.get("Alias")
    if name and name.replace("-", ":").upper() == props["Address"].upper():
        name = None  # BlueZ's Alias defaults to the address
    return Reading(props["Address"], name, int(props["RSSI"]), clock(), transport="classic")


class BluezClassicSource:
    def __init__(self, queue: asyncio.Queue, clock=time.monotonic, bus_factory=None) -> None:
        self.queue, self.clock = queue, clock
        self.bus_factory = bus_factory  # tests inject a fake; default is the system bus
        self.devices: dict[str, dict] = {}
        self.bus = None
        self.adapter = None

    async def _call(self, path, iface, member, signature="", body=()):
        from dbus_fast import Message, MessageType

        reply = await self.bus.call(Message(destination="org.bluez", path=path, interface=iface,
                                            member=member, signature=signature, body=list(body)))
        if reply.message_type == MessageType.ERROR:
            raise RuntimeError(f"{reply.error_name}: {' '.join(map(str, reply.body))}")
        return reply.body

    def _on_message(self, msg) -> None:
        from dbus_fast import MessageType

        if msg.message_type != MessageType.SIGNAL:
            return
        if msg.member == "InterfacesAdded":
            path, ifaces = msg.body
            if DEVICE_IFACE in ifaces:
                self.devices[path] = unwrap(ifaces[DEVICE_IFACE])
                self._push(path)
        elif msg.member == "PropertiesChanged" and msg.body and msg.body[0] == DEVICE_IFACE:
            props = self.devices.setdefault(msg.path, {})
            props.update(unwrap(msg.body[1]))
            if "RSSI" in msg.body[1]:
                self._push(msg.path)

    def _push(self, path) -> None:
        r = bluez_reading(self.devices[path], self.clock)
        if r:
            self.queue.put_nowait(r)

    async def start(self) -> None:
        try:
            from dbus_fast import BusType, Message, Variant

            if self.bus_factory is None:
                from dbus_fast.aio import MessageBus

                self.bus_factory = lambda: MessageBus(bus_type=BusType.SYSTEM)
            self.bus = await self.bus_factory().connect()
            (objs,) = await self._call("/", "org.freedesktop.DBus.ObjectManager", "GetManagedObjects")
            adapters = {p: unwrap(i[ADAPTER_IFACE]) for p, i in objs.items() if ADAPTER_IFACE in i}
            if not adapters:
                raise AdapterError(NO_ADAPTER_MSG.replace("LE ", ""))
            self.adapter, info = sorted(adapters.items())[0]
            if not info.get("Powered", True):
                raise translate(RuntimeError("powered off"))
            for p, i in objs.items():
                if DEVICE_IFACE in i:
                    self.devices[p] = unwrap(i[DEVICE_IFACE])
            await self.bus.call(Message(
                destination="org.freedesktop.DBus", path="/org/freedesktop/DBus",
                interface="org.freedesktop.DBus", member="AddMatch", signature="s",
                body=["type='signal',sender='org.bluez'"]))
            self.bus.add_message_handler(self._on_message)
            await self._call(self.adapter, ADAPTER_IFACE, "SetDiscoveryFilter", "a{sv}",
                             [{"Transport": Variant("s", "bredr"),
                               "DuplicateData": Variant("b", True)}])
            await self._call(self.adapter, ADAPTER_IFACE, "StartDiscovery")
        except AdapterError:
            raise
        except ImportError as e:
            raise AdapterError("Classic scanning on Linux needs `dbus-fast` (pip install dbus-fast).",
                               2) from e
        except Exception as e:
            raise translate(e) from e

    async def stop(self) -> None:
        if self.bus is None:
            return
        try:
            if self.adapter:
                await self._call(self.adapter, ADAPTER_IFACE, "StopDiscovery")
        except Exception:
            pass
        self.bus.disconnect()


def classic_source(queue: asyncio.Queue, clock=time.monotonic):
    if sys.platform == "win32":
        return WinClassicSource(queue, clock)
    if sys.platform.startswith("linux"):
        return BluezClassicSource(queue, clock)
    raise AdapterError("Classic Bluetooth (BR/EDR) scanning isn't supported on this platform "
                       "(macOS only exposes inquiry through IOBluetooth). Use --transport le.", 2)


class MultiSource:
    """Run several sources into the same queue (e.g. LE + classic)."""

    def __init__(self, *sources) -> None:
        self.sources = sources

    async def start(self) -> None:
        started = []
        try:
            for s in self.sources:
                await s.start()
                started.append(s)
        except BaseException:
            for s in started:
                await s.stop()
            raise

    async def stop(self) -> None:
        for s in self.sources:
            await s.stop()
