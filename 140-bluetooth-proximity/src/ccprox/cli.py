"""ccprox command line: survey (default), --calibrate T, --hunt T, --serve HOST:PORT."""

from __future__ import annotations

import argparse
import asyncio
import csv
import errno
import os
import sys
import time
from typing import Callable, TextIO

from . import __version__, render
from .proximity import (DEFAULT_FAR_DBM, DEFAULT_NEAR_DBM, DEFAULT_NEAR_M, DEFAULT_PATH_LOSS,
                        Calibration, default_store, estimate_distance, load_calibrations,
                        reference_from_samples, save_calibration)
from .scanner import AdapterError, BleakSource, SimSource
from .sound import Clicker
from .stations import RemoteSource, StationError, StationServer, locate, parse_hostport
from .tracker import DEFAULT_TIMEOUT_S, DEFAULT_WINDOW, DeviceState, Reading, Tracker, matches

EXIT_OK = 0
EXIT_NOT_FOUND = 1
EXIT_USAGE = 2
EXIT_INTERRUPTED = 130

EPILOG = f"""\
modes:
  (default)          survey: live table of nearby devices, strongest first
  --calibrate T      hold T at arm's length; record its RSSI as the "near" reference
  --hunt T           track one device (name or address) with warmer/colder feedback
  --serve HOST:PORT  run as a remote scanning station for another machine's --hunt --remote

transports:
  --transport le       Bluetooth Low Energy advertisements (default; fast updates)
  --transport classic  classic BR/EDR inquiry (Windows, Linux; slow, discoverable/paired only)
  --transport both     both, shown as separate LE / BR rows

bands (relative to the near / far references):
  IMMEDIATE  >= near      NEAR  near..midpoint      MEDIUM  midpoint..far      FAR  < far

distance:
  log-distance path loss model anchored at the near reference:
    d = near_distance * 10 ^ ((near - rssi) / (10 * path_loss))
  {render.DISCLAIMER}
  A single radio measures signal strength only: it tells you HOW FAR, never WHICH WAY.
  Walk, then watch whether the signal rises or falls.

direction (two stations):
  machine B:  ccprox --serve 0.0.0.0:7140
  machine A:  ccprox --hunt T --remote B-HOST:7140 --baseline 4
  Each station's distance is a circle; their intersection places the target along
  the A->B line and +/- some metres to one side. Which side is unknowable with two.

exit status:
  0 ok, 1 target not found, 2 usage error, 3 adapter missing/off,
  4 permission denied, 5 remote station unreachable
"""


def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        prog="ccprox",
        description="Bluetooth proximity finder: survey, calibrate and hunt by RSSI.",
        epilog=EPILOG, formatter_class=argparse.RawDescriptionHelpFormatter)
    mode = p.add_mutually_exclusive_group()
    mode.add_argument("--hunt", metavar="TARGET", help="track one device by name or address")
    mode.add_argument("--calibrate", metavar="TARGET", help="record the near reference for a device")
    mode.add_argument("--serve", metavar="HOST:PORT",
                      help="act as a remote station, streaming readings to --remote clients")
    p.add_argument("--transport", choices=("le", "classic", "both"), default="le",
                   help="radio(s) to scan (default le)")
    p.add_argument("--near", type=float, help=f"near (arm's length) reference dBm (default: "
                   f"saved calibration, else {DEFAULT_NEAR_DBM:g})")
    p.add_argument("--far", type=float, default=DEFAULT_FAR_DBM,
                   help=f"far reference dBm (default {DEFAULT_FAR_DBM:g})")
    p.add_argument("--interval", type=float, default=1.0, help="survey refresh seconds (default 1)")
    p.add_argument("--timeout", type=float, default=DEFAULT_TIMEOUT_S,
                   help="seconds without a reading before a device is dropped/lost (default 10)")
    p.add_argument("--start-timeout", type=float, default=30.0,
                   help="hunt/calibrate: give up if the target is never seen (default 30)")
    p.add_argument("--window", type=int, default=DEFAULT_WINDOW, help="readings to average (default 5)")
    p.add_argument("--dedupe-ms", type=float, default=200.0,
                   help="identical RSSI repeated within this many ms is a cached duplicate (default 200)")
    p.add_argument("--path-loss", type=float, default=DEFAULT_PATH_LOSS,
                   help="path-loss exponent n: 2 free space, 2.5-4 indoors (default 2)")
    p.add_argument("--near-distance", type=float, default=DEFAULT_NEAR_M,
                   help="metres at which the near reference applies (default 0.6, arm's length)")
    p.add_argument("--seconds", type=float, default=5.0, help="calibrate: sampling period (default 5)")
    p.add_argument("--yes", "-y", action="store_true", help="calibrate: don't wait for Enter")
    p.add_argument("--no-save", action="store_true", help="calibrate: don't persist the result")
    p.add_argument("--calibration-file", help=f"calibration store (default {default_store()})")
    p.add_argument("--sound", action="store_true", help="hunt: Geiger-counter clicks, faster when closer")
    p.add_argument("--remote", metavar="HOST:PORT",
                   help="hunt: second station (a `ccprox --serve`) for direction finding")
    p.add_argument("--baseline", type=float, metavar="M",
                   help="hunt --remote: distance in metres between this machine (A) and the station (B)")
    p.add_argument("--remote-near", type=float, metavar="DBM",
                   help="hunt --remote: station B's near reference (default: same as A's)")
    p.add_argument("--station-name", default=None, help="serve: name announced to clients")
    p.add_argument("--verbose", "-v", action="store_true", help="show raw vs counted readings")
    p.add_argument("--duration", type=float, help="stop after this many seconds")
    p.add_argument("--log", metavar="CSV", help="append every reading to a CSV file")
    p.add_argument("--simulate", action="store_true", help="use simulated devices instead of the radio")
    p.add_argument("--sim-offset", type=float, default=0.0, metavar="DB",
                   help="simulate: add this many dB to every simulated RSSI (fake a second position)")
    p.add_argument("--version", action="version", version=f"ccprox {__version__}")
    return p


class App:
    def __init__(self, args, out: TextIO, err: TextIO, source_factory: Callable,
                 clock=time.monotonic, sink=None) -> None:
        self.a = args
        self.out, self.err = out, err
        self.clock = clock
        self.source_factory = source_factory
        self.queue: asyncio.Queue = asyncio.Queue()
        self.source = None
        self.remote: RemoteSource | None = None
        self.tracker = Tracker(args.window, args.dedupe_ms / 1000, args.timeout)
        self.remote_tracker = Tracker(args.window, args.dedupe_ms / 1000, args.timeout)
        self.sink = sink
        self.tty = getattr(out, "isatty", lambda: False)()
        self._block = 0  # lines in the live (redrawn in place) block on a tty
        self._log = None
        self._logw = None

    def calibration(self, target: str | None = None) -> Calibration:
        near = self.a.near
        if near is None and target:
            saved = load_calibrations(self._store())
            near = saved.get(target.lower())
            if near is None:
                # target may be a name while the store also keys by address, or vice versa
                for st in self.tracker.devices.values():
                    if matches(target, st.address, st.name):
                        near = saved.get(st.address.lower()) or (st.name and saved.get(st.name.lower()))
                        break
        return Calibration(DEFAULT_NEAR_DBM if near is None else near, self.a.far)

    def _store(self):
        from pathlib import Path
        return Path(self.a.calibration_file) if self.a.calibration_file else None

    async def next_reading(self, deadline: float) -> Reading | None:
        wait = deadline - self.clock()
        if wait <= 0:
            return None
        try:
            r = await asyncio.wait_for(self.queue.get(), wait)
        except asyncio.TimeoutError:
            return None
        self.log(r)
        return r

    def log(self, r: Reading) -> None:
        if not self.a.log:
            return
        if self._logw is None:
            new = not os.path.exists(self.a.log) or os.path.getsize(self.a.log) == 0
            self._log = open(self.a.log, "a", newline="", encoding="utf-8")
            self._logw = csv.writer(self._log)
            if new:
                self._logw.writerow(["time", "station", "transport", "address", "name", "rssi"])
        self._logw.writerow([f"{time.time():.3f}", r.station, r.transport, r.address,
                             r.name or "", r.rssi])

    def end_time(self) -> float:
        return self.clock() + self.a.duration if self.a.duration else float("inf")

    def say(self, s: str = "") -> None:
        print(s, file=self.out, flush=True)

    def show(self, *lines: str) -> None:
        """Redraw the live block in place on a terminal; print plainly otherwise."""
        if not self.tty:
            for ln in lines:
                self.say(ln)
            return
        buf = "\r" + ("\x1b[B" * 0)
        buf += "\n".join("\x1b[K" + ln for ln in lines)
        if len(lines) > 1:
            buf += f"\x1b[{len(lines) - 1}A\r"
        self.out.write(buf)
        self.out.flush()
        self._block = len(lines)

    def newline(self) -> None:
        """Leave the live block so a normal line can be printed below it."""
        if self.tty and self._block:
            self.out.write("\n" * self._block)
            self._block = 0

    async def run(self) -> int:
        self.source = self.source_factory(self.queue, self.clock)
        await self.source.start()
        try:
            if self.a.remote:
                host, port = parse_hostport(self.a.remote, "127.0.0.1")
                self.remote = RemoteSource(self.queue, host, port, "B", self.clock)
                await self.remote.start()
                self.say(f"Connected to station B {self.remote.remote_name!r} at {host}:{port}")
            if self.a.hunt:
                return await self.hunt(self.a.hunt)
            if self.a.calibrate:
                return await self.calibrate(self.a.calibrate)
            if self.a.serve:
                return await self.serve()
            return await self.survey()
        finally:
            if self.remote:
                await self.remote.stop()
            await self.source.stop()
            if self._log:
                self._log.close()

    async def survey(self) -> int:
        cal = self.calibration()
        end = self.end_time()
        next_draw = self.clock() + self.a.interval
        while True:
            r = await self.next_reading(min(next_draw, end))
            if r:
                self.tracker.ingest(r)
                continue
            now = self.clock()
            self.tracker.expire(now)
            frame = render.survey(self.tracker.ranked(now), cal, now, self.a.verbose,
                                  self.tracker.totals(), show_transport=self.a.transport != "le")
            if self.tty:
                self.out.write("\x1b[H\x1b[2J")  # home + clear: redraw in place
            else:
                self.out.write("\n")
            self.say(frame)
            if now >= end:
                return EXIT_OK
            next_draw = now + self.a.interval

    async def serve(self) -> int:
        host, port = parse_hostport(self.a.serve)
        server = StationServer(host, port, self.a.station_name or _hostname())
        await server.start()
        self.say(f"Station {server.name!r} listening on {host}:{server.port} "
                 f"({self.a.transport}{', simulated' if self.a.simulate else ''}) — Ctrl+C to stop")
        end = self.end_time()
        next_status = self.clock() + max(self.a.interval, 5.0)
        try:
            while True:
                r = await self.next_reading(min(next_status, end))
                if r:
                    server.publish(r)
                while server.events:
                    self.say(server.events.pop(0))
                now = self.clock()
                if now >= next_status:
                    if self.a.verbose:
                        self.say(f"{server.sent} readings relayed to {len(server.clients)} client(s)")
                    next_status = now + max(self.a.interval, 5.0)
                if now >= end:
                    return EXIT_OK
        finally:
            await server.stop()

    async def await_target(self, target: str) -> Reading | None:
        """Wait up to --start-timeout for the first local reading of the target."""
        self.say(f"Looking for {target!r} (up to {self.a.start_timeout:g}s)…")
        deadline = self.clock() + self.a.start_timeout
        while True:
            r = await self.next_reading(deadline)
            if r is None:
                print(f"ccprox: target {target!r} not found within {self.a.start_timeout:g}s. "
                      "Run without --hunt to survey what is in range.", file=self.err)
                return None
            if r.station != "A":
                self.remote_tracker.ingest(r)
            elif matches(target, r.address, r.name):
                return r

    def remote_state(self, target: str, st: DeviceState) -> DeviceState | None:
        """The target as station B sees it: same address+transport, else same name."""
        exact = self.remote_tracker.devices.get(st.key)
        if exact:
            return exact
        for d in self.remote_tracker.devices.values():
            if d.transport == st.transport and (matches(target, d.address, d.name)
                                                or (st.name and d.name == st.name)):
                return d
        return None

    def direction_line(self, target: str, st: DeviceState, cal: Calibration,
                       cal_b: Calibration, now: float) -> str:
        if self.remote and not self.remote.connected:
            return "B: station disconnected — direction unavailable"
        sb = self.remote_state(target, st)
        if sb is None:
            return "B: station has not heard the target yet"
        if not self.remote_tracker.is_live(sb, now):
            return f"B: station lost the target ({now - sb.last_raw_t:.0f}s ago)"
        if not self.tracker.is_live(st, now):
            return "A: this machine lost the target — direction unavailable"
        n, d0 = self.a.path_loss, self.a.near_distance
        d_a = estimate_distance(st.smoothed, cal.near, d0, n)
        d_b = estimate_distance(sb.smoothed, cal_b.near, d0, n)
        fix = locate(d_a, d_b, self.a.baseline)
        return render.direction(fix, self.a.baseline, d_a, d_b, st.smoothed, sb.smoothed)

    async def hunt(self, target: str) -> int:
        first = await self.await_target(target)
        if first is None:
            return EXIT_NOT_FOUND
        st = self.tracker.ingest(first)
        cal = self.calibration(target)
        cal_b = Calibration(self.a.remote_near if self.a.remote_near is not None else cal.near,
                            self.a.far)
        kind = "" if st.transport == "le" else f" ({render.TRANSPORT[st.transport]})"
        self.say(f"Hunting {st.label} [{st.address}]{kind}  near {cal.near:g} dBm / far "
                 f"{cal.far:g} dBm{'  (sound on)' if self.a.sound else ''}  — Ctrl+C to stop")
        self.say(render.DISCLAIMER)
        if self.remote:
            self.say(f"Direction: A = this machine, B = station {self.remote.remote_name!r}, "
                     f"{self.a.baseline:g} m apart.")
        clicker = None
        if self.a.sound:
            kw = {"sink": self.sink} if self.sink else {}
            clicker = Clicker(cal, self.a.timeout, clock=self.clock, **kw)
            clicker.update(st.smoothed)
            clicker.start()

        def frame(verbose=self.a.verbose):
            line = render.hunt(st, cal, self.a.near_distance, self.a.path_loss, verbose)
            if not self.remote:
                return (line,)
            return line, self.direction_line(target, st, cal, cal_b, self.clock())

        self.show(*frame())
        end = self.end_time()
        is_lost = False
        last_dir = None
        try:
            while True:
                r = await self.next_reading(min(self.clock() + 0.25, end))
                now = self.clock()
                if r and r.station != "A":
                    if self.remote_tracker.ingest(r) and self.remote_state(target, st) is not None:
                        f = frame()
                        if self.tty or f[-1] != last_dir:
                            self.show(*f)
                            last_dir = f[-1]
                elif r and r.key == st.key:
                    if self.tracker.ingest(r) is None:
                        if self.a.verbose:
                            self.show(*frame(True))
                        continue
                    if is_lost:
                        is_lost = False
                        self.newline()
                        self.say(f"Target reacquired: {st.label}")
                    if clicker:
                        clicker.update(st.smoothed)
                    self.show(*frame())
                elif not is_lost and not self.tracker.is_live(st, now):
                    is_lost = True
                    self.newline()
                    self.say(render.lost(st, now - st.last_raw_t))
                if now >= end:
                    self.newline()
                    return EXIT_OK
        finally:
            if clicker:
                await clicker.stop()

    async def calibrate(self, target: str) -> int:
        self.say(f"Calibration: hold {target!r} at arm's length (~{self.a.near_distance:g} m) "
                 "from this computer, in clear line of sight.")
        if not self.a.yes and sys.stdin and sys.stdin.isatty():
            await asyncio.to_thread(input, "Press Enter when ready… ")
            while not self.queue.empty():  # discard readings taken while positioning
                self.queue.get_nowait()
        first = await self.await_target(target)
        if first is None:
            return EXIT_NOT_FOUND
        st = self.tracker.ingest(first)
        samples = [first.rssi]
        self.say(f"Sampling {st.label} [{st.address}] for {self.a.seconds:g}s…")
        deadline = self.clock() + self.a.seconds
        while (r := await self.next_reading(deadline)) is not None:
            if r.station == "A" and r.key == st.key and self.tracker.ingest(r):
                samples.append(r.rssi)
        if len(samples) < 3:
            print(f"ccprox: only {len(samples)} reading(s) from {st.label}; need at least 3. "
                  "Move closer or sample for longer (--seconds).", file=self.err)
            return EXIT_NOT_FOUND
        near = reference_from_samples(samples)
        self.say(f"{len(samples)} readings, min {min(samples)} / max {max(samples)} dBm  →  "
                 f"near reference {near:g} dBm")
        if near <= self.a.far:
            print(f"ccprox: near reference {near:g} dBm is not stronger than far {self.a.far:g} dBm; "
                  "was the device really at arm's length?", file=self.err)
            return EXIT_USAGE
        if not self.a.no_save:
            path = save_calibration(st.address, near, self._store())
            if st.name:
                save_calibration(st.name, near, self._store())
            self.say(f"Saved to {path}; `--hunt {target}` will use it automatically.")
        self.say(f"Bands now: IMMEDIATE ≥ {near:g}, NEAR ≥ {(near + self.a.far) / 2:g}, "
                 f"MEDIUM ≥ {self.a.far:g}, FAR below.")
        return EXIT_OK


def _hostname() -> str:
    import socket
    return socket.gethostname()


def _validate(p: argparse.ArgumentParser, a) -> None:
    for name in ("interval", "timeout", "start_timeout", "seconds", "path_loss", "near_distance"):
        if getattr(a, name) <= 0:
            p.error(f"--{name.replace('_', '-')} must be positive")
    if a.window < 1:
        p.error("--window must be >= 1")
    if a.dedupe_ms < 0:
        p.error("--dedupe-ms must be >= 0")
    if a.duration is not None and a.duration <= 0:
        p.error("--duration must be positive")
    if a.near is not None and a.near <= a.far:
        p.error(f"--near ({a.near:g}) must be stronger (less negative) than --far ({a.far:g})")
    if a.remote_near is not None and a.remote_near <= a.far:
        p.error(f"--remote-near ({a.remote_near:g}) must be stronger than --far ({a.far:g})")
    if a.remote:
        if not a.hunt:
            p.error("--remote only works with --hunt")
        if a.baseline is None or a.baseline <= 0:
            p.error("--remote needs --baseline M (metres between the two stations, > 0)")
    elif a.baseline is not None or a.remote_near is not None:
        p.error("--baseline / --remote-near need --remote")
    for opt in ("serve", "remote"):
        if getattr(a, opt):
            try:
                parse_hostport(getattr(a, opt))
            except ValueError as e:
                p.error(f"--{opt}: {e}")


def default_source(a) -> Callable:
    transports = ("le", "classic") if a.transport == "both" else (a.transport,)
    if a.simulate:
        return lambda q, clock: SimSource(q, clock=clock, transports=transports,
                                          offset=a.sim_offset)

    def make(q, clock):
        from .classic import MultiSource, classic_source

        parts = []
        if "le" in transports:
            parts.append(BleakSource(q, clock=clock))
        if "classic" in transports:
            parts.append(classic_source(q, clock))
        return parts[0] if len(parts) == 1 else MultiSource(*parts)

    return make


def main(argv: list[str] | None = None, out: TextIO | None = None, err: TextIO | None = None,
         source_factory: Callable | None = None, clock=time.monotonic, sink=None) -> int:
    out = out or sys.stdout
    err = err or sys.stderr
    for stream in (out, err):
        if hasattr(stream, "reconfigure") and not stream.isatty():
            stream.reconfigure(encoding="utf-8", errors="replace")
    if sys.platform == "win32" and getattr(out, "isatty", lambda: False)():
        os.system("")  # enable ANSI escape handling in the Windows console
    p = build_parser()
    a = p.parse_args(argv)
    _validate(p, a)
    app = App(a, out, err, source_factory or default_source(a), clock, sink)
    try:
        return asyncio.run(app.run())
    except (AdapterError, StationError) as e:
        print(f"ccprox: {e}", file=err)
        return e.exit_code
    except KeyboardInterrupt:
        out.write("\n")
        return EXIT_INTERRUPTED
    except OSError as e:
        # Reader went away (`| head`); Windows reports a closed pipe as EINVAL.
        if not isinstance(e, BrokenPipeError) and e.errno not in (errno.EPIPE, errno.EINVAL):
            raise
        try:
            os.dup2(os.open(os.devnull, os.O_WRONLY), out.fileno())
        except (OSError, ValueError, AttributeError):
            pass
        return EXIT_OK
