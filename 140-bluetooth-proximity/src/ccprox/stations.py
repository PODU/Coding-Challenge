"""Two-station direction finding.

One radio measures signal strength, i.e. distance — never bearing. A second
scanner at a known distance (the *baseline*) adds a second distance, and two
distances from two known points pin the target to the intersection of two
circles: a position along the A→B line plus an offset either side of it. Which
side cannot be told apart (mirror ambiguity) — a third station would fix that.

Wire protocol: a station (``ccprox --serve HOST:PORT``) streams every reading
it hears as one JSON object per line over TCP:

    {"ccprox": 1, "station": "kitchen"}                 (hello, first line)
    {"address": "AA:..", "name": "Tag", "rssi": -61, "transport": "le"}
"""

from __future__ import annotations

import asyncio
import json
import math
import time
from dataclasses import dataclass

from .tracker import Reading

PROTOCOL = 1
EXIT_STATION = 5
CONNECT_TIMEOUT_S = 5.0


class StationError(Exception):
    exit_code = EXIT_STATION


def parse_hostport(s: str, default_host: str = "0.0.0.0") -> tuple[str, int]:
    host, sep, port = s.rpartition(":")
    if not sep:
        host, port = default_host, s
    try:
        p = int(port)
    except ValueError:
        raise ValueError(f"bad port in {s!r}") from None
    if not 0 <= p <= 65535:
        raise ValueError(f"port out of range in {s!r}")
    return host.strip("[]") or default_host, p


def encode(r: Reading) -> bytes:
    return (json.dumps({"address": r.address, "name": r.name, "rssi": r.rssi,
                        "transport": r.transport}) + "\n").encode()


def decode(line: bytes, station: str, t: float) -> Reading | None:
    try:
        d = json.loads(line)
        if not isinstance(d, dict) or "rssi" not in d:
            return None  # hello or unknown message
        return Reading(str(d["address"]), d.get("name") or None, int(d["rssi"]), t,
                       transport=d.get("transport", "le"), station=station)
    except (ValueError, KeyError, TypeError):
        return None


class StationServer:
    """Fan readings out to every connected client."""

    def __init__(self, host: str, port: int, name: str = "station") -> None:
        self.host, self.port, self.name = host, port, name
        self.clients: set[asyncio.StreamWriter] = set()
        self.server: asyncio.base_events.Server | None = None
        self.sent = 0
        self.events: list[str] = []  # connect/disconnect notices for the UI

    async def start(self) -> None:
        try:
            self.server = await asyncio.start_server(self._client, self.host, self.port)
        except OSError as e:
            raise StationError(f"cannot listen on {self.host}:{self.port}: {e.strerror or e}") from e
        self.port = self.server.sockets[0].getsockname()[1]

    async def _client(self, reader: asyncio.StreamReader, writer: asyncio.StreamWriter) -> None:
        peer = writer.get_extra_info("peername")
        writer.write((json.dumps({"ccprox": PROTOCOL, "station": self.name}) + "\n").encode())
        self.clients.add(writer)
        self.events.append(f"client connected: {peer[0]}:{peer[1]}")
        try:
            await reader.read()  # clients never send; EOF means they left
        finally:
            self.clients.discard(writer)
            self.events.append(f"client disconnected: {peer[0]}:{peer[1]}")
            writer.close()

    def publish(self, r: Reading) -> None:
        data = encode(r)
        for w in list(self.clients):
            if w.is_closing():
                self.clients.discard(w)
                continue
            w.write(data)
        self.sent += 1

    async def stop(self) -> None:
        for w in list(self.clients):
            w.close()
        if self.server:
            self.server.close()
            await self.server.wait_closed()


class RemoteSource:
    """Connects to a station and feeds its readings, tagged ``station``, into the queue."""

    def __init__(self, queue: asyncio.Queue, host: str, port: int, station: str = "B",
                 clock=time.monotonic) -> None:
        self.queue, self.host, self.port, self.station, self.clock = queue, host, port, station, clock
        self.remote_name: str | None = None
        self.connected = False
        self._task: asyncio.Task | None = None
        self._writer = None

    async def start(self) -> None:
        try:
            reader, self._writer = await asyncio.wait_for(
                asyncio.open_connection(self.host, self.port), CONNECT_TIMEOUT_S)
            hello = json.loads(await asyncio.wait_for(reader.readline(), CONNECT_TIMEOUT_S))
            if not isinstance(hello, dict) or hello.get("ccprox") != PROTOCOL:
                raise ValueError("not a ccprox station")
        except (OSError, asyncio.TimeoutError, ValueError) as e:
            if self._writer:
                self._writer.close()
            why = "timed out" if isinstance(e, asyncio.TimeoutError) else (
                getattr(e, "strerror", None) or str(e))
            raise StationError(f"cannot reach station {self.host}:{self.port} ({why}). "
                               "Is `ccprox --serve` running there, and the port open?") from e
        self.remote_name = hello.get("station")
        self.connected = True
        self._task = asyncio.create_task(self._run(reader))

    async def _run(self, reader: asyncio.StreamReader) -> None:
        try:
            while line := await reader.readline():
                r = decode(line, self.station, self.clock())
                if r:
                    self.queue.put_nowait(r)
        except (OSError, asyncio.IncompleteReadError):
            pass
        self.connected = False

    async def stop(self) -> None:
        if self._task:
            self._task.cancel()
            try:
                await self._task
            except asyncio.CancelledError:
                pass
        if self._writer:
            self._writer.close()


@dataclass(frozen=True)
class Fix:
    x: float  # metres along A→B (A at 0, B at baseline)
    y: float  # metres either side of the line (sign unknowable)
    consistent: bool  # False when the two circles don't meet; (x, 0) is then the closest fit


def locate(d_a: float, d_b: float, baseline: float) -> Fix:
    """Intersect circle(A, d_a) with circle(B, d_b), A=(0,0), B=(baseline,0).

        x = (dA² − dB² + L²) / 2L,   y = √(dA² − x²)

    Noisy RSSI often gives circles that miss each other (too far apart, or one
    inside the other). Then the point on the A–B line whose distances best
    match (least squares) is returned with ``consistent=False``."""
    if baseline <= 0:
        raise ValueError("baseline must be positive")
    L = baseline
    x = (d_a ** 2 - d_b ** 2 + L ** 2) / (2 * L)
    y2 = d_a ** 2 - x ** 2
    if y2 >= 0:
        return Fix(x, math.sqrt(y2), True)

    def err(x: float) -> float:
        return (abs(x) - d_a) ** 2 + (abs(x - L) - d_b) ** 2

    # On each stretch of the line the error is a parabola; take each vertex,
    # clamped to its stretch, and keep the best.
    candidates = [min(0.0, (L - d_a - d_b) / 2),        # beyond A
                  min(L, max(0.0, (L + d_a - d_b) / 2)),  # between
                  max(L, (d_a + d_b + L) / 2)]            # beyond B
    return Fix(min(candidates, key=err), 0.0, False)


def describe(fix: Fix, baseline: float) -> str:
    if fix.x < 0:
        return f"beyond A ({-fix.x:.1f} m past A, away from B)"
    if fix.x > baseline:
        return f"beyond B ({fix.x - baseline:.1f} m past B, away from A)"
    if abs(fix.x - baseline / 2) < baseline * 0.1:
        return "between A and B, about midway"
    nearer = "A" if fix.x < baseline / 2 else "B"
    return f"between A and B, nearer {nearer}"
