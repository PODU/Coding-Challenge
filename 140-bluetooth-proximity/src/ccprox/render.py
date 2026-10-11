"""Pure text rendering for survey and hunt output."""

from __future__ import annotations

from .proximity import Calibration, estimate_distance
from .stations import Fix, describe
from .tracker import DeviceState

DISCLAIMER = ("Distances are rough RSSI estimates (walls, bodies and antenna orientation easily "
              "cause 2x errors). One radio gives distance, not direction.")


def bar(fraction: float, width: int = 20) -> str:
    n = round(max(0.0, min(1.0, fraction)) * width)
    return "█" * n + "·" * (width - n)


def _trunc(s: str, w: int) -> str:
    return s if len(s) <= w else s[: w - 1] + "…"


TRANSPORT = {"le": "LE", "classic": "BR"}


def survey(devices: list[DeviceState], cal: Calibration, now: float, verbose: bool = False,
           totals: tuple[int, int] | None = None, show_transport: bool = False) -> str:
    head = f"{'#':>3}  {'NAME':<24} {'ADDRESS':<17} {'RSSI':>5} {'AVG':>6}  {'BAND':<9} {'SIGNAL':<20} {'AGE':>5}"
    if show_transport:
        head = head.replace("ADDRESS          ", "ADDRESS           TYPE", 1)
    if verbose:
        head += f" {'RAW':>6} {'CNT':>6}"
    lines = [head]
    for i, d in enumerate(devices, 1):
        addr = f"{d.address:<17}" + (f" {TRANSPORT.get(d.transport, d.transport):<4}" if show_transport else "")
        row = (f"{i:>3}  {_trunc(d.label, 24):<24} {addr} {d.latest:>5} {d.smoothed:>6.1f}  "
               f"{cal.band(d.smoothed):<9} {bar(cal.fraction(d.smoothed))} {now - d.last_raw_t:>4.1f}s")
        if verbose:
            row += f" {d.raw:>6} {d.counted:>6}"
        lines.append(row)
    if not devices:
        lines.append("     (no devices in range yet…)")
    foot = f"{len(devices)} device(s)  ·  near {cal.near:g} dBm / far {cal.far:g} dBm"
    if verbose and totals:
        raw, counted = totals
        foot += f"  ·  readings raw {raw}, counted {counted}, discarded {raw - counted}"
    lines.append(foot)
    return "\n".join(lines)


def hunt(d: DeviceState, cal: Calibration, near_m: float, path_loss: float,
         verbose: bool = False) -> str:
    s = d.smoothed
    dist = estimate_distance(s, cal.near, near_m, path_loss)
    line = (f"{_trunc(d.label, 20):<20} {cal.band(s):<9} {bar(cal.fraction(s))} "
            f"RSSI {d.latest:>4} dBm  avg {s:6.1f}  ~{dist:5.1f} m")
    if verbose:
        line += f"  raw {d.raw} counted {d.counted}"
    return line


def lost(d: DeviceState, ago: float) -> str:
    return f"!! TARGET LOST: {d.label} — no signal for {ago:.0f}s (last avg {d.smoothed:.1f} dBm)"


def strip(fix: Fix, baseline: float, width: int = 31) -> str:
    """A├──●────┤B on a scale from −L/2 to 1.5·L, so 'beyond' positions still show."""
    lo, hi = -baseline / 2, baseline * 1.5

    def col(x: float) -> int:
        return round((min(max(x, lo), hi) - lo) / (hi - lo) * (width - 1))

    cells = [" "] * width
    a, b = col(0), col(baseline)
    for i in range(a, b + 1):
        cells[i] = "─"
    cells[a], cells[b] = "├", "┤"
    cells[a - 1], cells[b + 1] = "A", "B"
    cells[col(fix.x)] = "●"
    return "".join(cells)


def direction(fix: Fix, baseline: float, d_a: float, d_b: float, rssi_a: float,
              rssi_b: float) -> str:
    where = describe(fix, baseline)
    side = (f"±{fix.y:.1f} m off the A–B line (side unknown)" if fix.consistent
            else "circles don't meet: nearest fit on the line")
    return (f"[{strip(fix, baseline)}]  {where}; {fix.x:.1f} m along A→B, {side}"
            f"  (A {rssi_a:.0f} dBm ~{d_a:.1f} m, B {rssi_b:.0f} dBm ~{d_b:.1f} m)")
