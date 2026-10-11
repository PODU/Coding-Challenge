"""Per-device RSSI state: rolling window, smoothing, de-duplication, expiry.

Everything here is pure and clock-injected (callers pass ``t``/``now``), so the
behaviour is fully testable without a radio.
"""

from __future__ import annotations

from collections import deque
from dataclasses import dataclass, field

DEFAULT_WINDOW = 5
DEFAULT_DEDUPE_S = 0.2
DEFAULT_TIMEOUT_S = 10.0


@dataclass(frozen=True)
class Reading:
    """One advertisement seen by the scanner."""

    address: str
    name: str | None
    rssi: int
    t: float  # monotonic seconds
    transport: str = "le"  # "le" or "classic" (BR/EDR)
    station: str = "A"  # which scanning station heard it ("A" = this machine)

    @property
    def key(self) -> str:
        """Tracker key: a dual-mode device seen over LE and classic is two rows."""
        return device_key(self.address, self.transport)


def device_key(address: str, transport: str = "le") -> str:
    return address.upper() if transport == "le" else f"{address.upper()}/{transport}"


@dataclass
class DeviceState:
    address: str
    name: str | None
    transport: str = "le"
    window: deque = field(default_factory=lambda: deque(maxlen=DEFAULT_WINDOW))
    last_rssi: int | None = None
    last_raw_t: float = 0.0  # time of last advertisement, counted or not
    last_counted_t: float = 0.0
    raw: int = 0  # every advertisement received
    counted: int = 0  # advertisements that survived de-duplication

    @property
    def key(self) -> str:
        return device_key(self.address, self.transport)

    @property
    def label(self) -> str:
        return self.name or self.address

    @property
    def smoothed(self) -> float:
        """Moving average over the rolling window."""
        return sum(self.window) / len(self.window)

    @property
    def latest(self) -> int:
        return self.window[-1]


class Tracker:
    """Ingests readings and keeps one :class:`DeviceState` per address."""

    def __init__(
        self,
        window: int = DEFAULT_WINDOW,
        dedupe_s: float = DEFAULT_DEDUPE_S,
        timeout_s: float = DEFAULT_TIMEOUT_S,
    ) -> None:
        if window < 1:
            raise ValueError("window must be >= 1")
        self.window = window
        self.dedupe_s = dedupe_s
        self.timeout_s = timeout_s
        self.devices: dict[str, DeviceState] = {}

    def ingest(self, r: Reading) -> DeviceState | None:
        """Record a reading. Returns the device state if the reading counted,
        or ``None`` if it was discarded as a cached duplicate.

        Operating systems often re-deliver the last cached advertisement; an
        identical RSSI arriving within ``dedupe_s`` of the previous one is
        treated as such a repeat and not added to the window.
        """
        key = r.key
        st = self.devices.get(key)
        if st is None:
            st = DeviceState(r.address, r.name, r.transport, deque(maxlen=self.window))
            self.devices[key] = st
        if r.name:
            st.name = r.name
        st.raw += 1
        dup = st.last_rssi == r.rssi and (r.t - st.last_raw_t) < self.dedupe_s
        st.last_rssi = r.rssi
        st.last_raw_t = r.t
        if dup:
            return None
        st.window.append(r.rssi)
        st.counted += 1
        st.last_counted_t = r.t
        return st

    def is_live(self, st: DeviceState, now: float) -> bool:
        return (now - st.last_raw_t) <= self.timeout_s

    def expire(self, now: float) -> list[DeviceState]:
        """Drop devices not heard from within the timeout; return them."""
        gone = [s for s in self.devices.values() if not self.is_live(s, now)]
        for s in gone:
            del self.devices[s.key]
        return gone

    def ranked(self, now: float) -> list[DeviceState]:
        """Live devices, strongest smoothed signal first."""
        live = [s for s in self.devices.values() if self.is_live(s, now)]
        return sorted(live, key=lambda s: (-s.smoothed, s.label.lower()))

    def totals(self) -> tuple[int, int]:
        """(raw, counted) summed over every tracked device."""
        return (
            sum(s.raw for s in self.devices.values()),
            sum(s.counted for s in self.devices.values()),
        )


def matches(target: str, address: str, name: str | None) -> bool:
    """Case-insensitive match on address, or on advertised name."""
    t = target.strip().lower()
    return t == address.lower() or (name is not None and t == name.strip().lower())
