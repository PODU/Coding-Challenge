"""Reading sources: the real BLE radio (bleak) and a deterministic simulator.

Both push :class:`Reading` objects onto an ``asyncio.Queue``. Adapter problems
are translated into :class:`AdapterError` with an actionable message instead of
a backend-specific stack trace.
"""

from __future__ import annotations

import asyncio
import math
import random
import sys
import time
from dataclasses import dataclass

from .tracker import Reading

EXIT_ADAPTER = 3  # no adapter / powered off
EXIT_PERMISSION = 4  # OS refused access

START_TIMEOUT_S = 10.0


class AdapterError(Exception):
    def __init__(self, message: str, exit_code: int = EXIT_ADAPTER) -> None:
        super().__init__(message)
        self.exit_code = exit_code


def _hint_power() -> str:
    if sys.platform == "win32":
        return "Turn Bluetooth on in Settings > Bluetooth & devices (or the Action Center), then retry."
    if sys.platform == "darwin":
        return "Turn Bluetooth on from Control Centre or System Settings > Bluetooth, then retry."
    return "Power the adapter on with `bluetoothctl power on` (and `rfkill unblock bluetooth` if blocked), then retry."


def _hint_permission() -> str:
    if sys.platform == "win32":
        return "Allow apps to use Bluetooth in Settings > Privacy & security > Bluetooth (and Radios), then retry."
    if sys.platform == "darwin":
        return ("Grant Bluetooth access to your terminal app in System Settings > Privacy & Security > "
                "Bluetooth, then restart the terminal.")
    return ("Your user may not be allowed to talk to BlueZ over D-Bus. Add yourself to the `bluetooth` "
            "group (then log out and in), or check the BlueZ D-Bus policy.")


NO_ADAPTER_MSG = "No Bluetooth LE adapter found. ccprox needs a BLE-capable (Bluetooth 4.0+) radio."


def translate(exc: BaseException) -> AdapterError:
    """Map whatever the backend raised onto a clear, actionable AdapterError."""
    reason = getattr(exc, "reason", None)
    rname = getattr(reason, "name", "") or ""
    text = f"{type(exc).__name__}: {exc}".lower()

    if rname in ("NO_BLUETOOTH", "NO_BLE_CENTRAL_ROLE") or any(
        s in text for s in ("no bluetooth adapters", "no adapter", "adapter not found",
                            "radio not found", "no default adapter")
    ):
        return AdapterError(NO_ADAPTER_MSG, EXIT_ADAPTER)
    if rname == "POWERED_OFF" or any(
        s in text for s in ("powered off", "poweredoff", "not ready", "notready", "radio is off",
                            "turned off", "rfkill")
    ):
        return AdapterError("Bluetooth adapter is powered off. " + _hint_power(), EXIT_ADAPTER)
    if rname.startswith("DENIED") or any(
        s in text for s in ("denied", "unauthorized", "not authorized", "accessdenied", "permission")
    ):
        return AdapterError("Permission to use Bluetooth was denied. " + _hint_permission(), EXIT_PERMISSION)
    return AdapterError(f"Bluetooth adapter error ({type(exc).__name__}: {exc}). " + _hint_power(),
                        EXIT_ADAPTER)


class BleakSource:
    """Continuous passive-ish BLE scan via bleak; duplicates are left on so
    every advertisement (and its RSSI) is reported."""

    def __init__(self, queue: asyncio.Queue, clock=time.monotonic) -> None:
        self.queue = queue
        self.clock = clock
        self._scanner = None

    def _callback(self, device, adv) -> None:
        name = adv.local_name or device.name
        self.queue.put_nowait(Reading(device.address, name, int(adv.rssi), self.clock()))

    async def start(self) -> None:
        try:
            from bleak import BleakScanner
        except ImportError as e:  # pragma: no cover - dependency missing
            raise AdapterError("The `bleak` package is not installed: pip install bleak", 2) from e
        try:
            kwargs = {"detection_callback": self._callback}
            if sys.platform == "darwin":
                kwargs["cb"] = {"use_bdaddr": False}
            self._scanner = BleakScanner(**kwargs)
            await asyncio.wait_for(self._scanner.start(), START_TIMEOUT_S)
        except asyncio.TimeoutError as e:
            raise AdapterError("Timed out starting the Bluetooth scan — the adapter is not responding. "
                               + _hint_power()) from e
        except AdapterError:
            raise
        except Exception as e:  # backend errors vary wildly by OS
            raise translate(e) from e

    async def stop(self) -> None:
        if self._scanner is not None:
            try:
                await self._scanner.stop()
            except Exception:
                pass


@dataclass
class SimDevice:
    name: str | None
    address: str
    base: float  # mean RSSI
    swing: float = 0.0  # amplitude of slow approach/retreat
    period: float = 30.0  # seconds per approach/retreat cycle
    gone: tuple[float, float] | None = None  # (start, end) of an out-of-range window, seconds
    rate: float = 8.0  # advertisements per second
    transport: str = "le"


DEFAULT_SIM = [
    SimDevice("SimTag", "AA:BB:CC:00:00:01", -60, swing=16, period=24, gone=(40, 52)),
    SimDevice("Headphones", "AA:BB:CC:00:00:02", -55),
    SimDevice("Watch", "AA:BB:CC:00:00:03", -68, swing=4, period=11),
    SimDevice(None, "AA:BB:CC:00:00:04", -84),
    SimDevice("Kettle", "AA:BB:CC:00:00:05", -77, gone=(8, 1e9)),
    SimDevice("SimSpeaker", "AA:BB:CC:00:00:06", -63, swing=6, period=40, rate=0.5,
              transport="classic"),
]


class SimSource:
    """Fake radio for demos and tests: noisy RSSI with slow sinusoidal motion,
    occasional OS-style cached duplicates, and scheduled dropouts."""

    def __init__(self, queue: asyncio.Queue, devices=None, seed: int = 140,
                 clock=time.monotonic, speed: float = 1.0, transports=("le",),
                 offset: float = 0.0) -> None:
        self.queue = queue
        self.devices = [d for d in (devices or DEFAULT_SIM) if d.transport in transports]
        self.offset = offset
        self.rng = random.Random(seed)
        self.clock = clock
        self.speed = speed
        self._task: asyncio.Task | None = None

    def rssi_at(self, d: SimDevice, elapsed: float) -> int:
        motion = d.swing * math.sin(2 * math.pi * elapsed / d.period)
        return round(d.base + self.offset + motion + self.rng.gauss(0, 2.5))

    async def _run(self) -> None:
        t0 = self.clock()
        nxt = {d.address: 0.0 for d in self.devices}
        last = {}
        repeat = set()  # devices owed a cached re-delivery of their last reading
        tick = 0.02
        while True:
            el = (self.clock() - t0) * self.speed
            for d in self.devices:
                if el < nxt[d.address]:
                    continue
                if d.address in repeat:
                    # the OS hands the same advertisement over again moments later
                    repeat.discard(d.address)
                    rssi = last[d.address]
                    nxt[d.address] = el + self.rng.expovariate(d.rate)
                else:
                    if d.gone and d.gone[0] <= el < d.gone[1]:
                        nxt[d.address] = el + self.rng.expovariate(d.rate)
                        continue
                    rssi = last[d.address] = self.rssi_at(d, el)
                    if self.rng.random() < 0.3:
                        repeat.add(d.address)
                        nxt[d.address] = el + 0.05
                    else:
                        nxt[d.address] = el + self.rng.expovariate(d.rate)
                self.queue.put_nowait(Reading(d.address, d.name, rssi, self.clock(),
                                              transport=d.transport))
            await asyncio.sleep(tick)

    async def start(self) -> None:
        self._task = asyncio.create_task(self._run())

    async def stop(self) -> None:
        if self._task:
            self._task.cancel()
            try:
                await self._task
            except asyncio.CancelledError:
                pass
