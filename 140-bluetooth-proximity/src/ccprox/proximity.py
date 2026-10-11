"""Turning RSSI into something a human can use: bands, distance, click tempo.

RSSI is a noisy, device-dependent proxy for distance. All of these mappings are
relative to two reference points: ``near`` (RSSI at arm's length, ideally
calibrated) and ``far`` (a typical "same room, across it" reading).
"""

from __future__ import annotations

import json
import os
import statistics
from dataclasses import dataclass
from pathlib import Path

DEFAULT_NEAR_DBM = -45.0
DEFAULT_FAR_DBM = -72.0
DEFAULT_NEAR_M = 0.6  # arm's length, the distance the near reference is taken at
DEFAULT_PATH_LOSS = 2.0  # free space; 2.5–4 indoors

# Ordered strongest to weakest.
BANDS = ("IMMEDIATE", "NEAR", "MEDIUM", "FAR")


@dataclass(frozen=True)
class Calibration:
    near: float = DEFAULT_NEAR_DBM
    far: float = DEFAULT_FAR_DBM

    def __post_init__(self) -> None:
        if not self.near > self.far:
            raise ValueError(
                f"near reference ({self.near:g} dBm) must be stronger (less negative) "
                f"than far reference ({self.far:g} dBm)"
            )

    @property
    def mid(self) -> float:
        return (self.near + self.far) / 2

    def band(self, rssi: float) -> str:
        """IMMEDIATE  ≥ near
        NEAR       between near and the near/far midpoint
        MEDIUM     between the midpoint and far
        FAR        weaker than far"""
        if rssi >= self.near:
            return "IMMEDIATE"
        if rssi >= self.mid:
            return "NEAR"
        if rssi >= self.far:
            return "MEDIUM"
        return "FAR"

    def fraction(self, rssi: float) -> float:
        """0.0 at/below far, 1.0 at/above near — for bars and tempo."""
        f = (rssi - self.far) / (self.near - self.far)
        return min(1.0, max(0.0, f))


def estimate_distance(
    rssi: float,
    near_rssi: float = DEFAULT_NEAR_DBM,
    near_m: float = DEFAULT_NEAR_M,
    n: float = DEFAULT_PATH_LOSS,
) -> float:
    """Log-distance path loss model, anchored at the calibrated near point:

        RSSI(d) = RSSI(d0) - 10·n·log10(d / d0)
        ⇒ d = d0 · 10^((RSSI(d0) - RSSI) / (10·n))
    """
    if n <= 0:
        raise ValueError("path-loss exponent must be positive")
    return near_m * 10 ** ((near_rssi - rssi) / (10 * n))


MIN_CLICK_S = 0.08
MAX_CLICK_S = 1.2


def click_interval(rssi: float, cal: Calibration) -> float:
    """Geiger-counter tempo: fast when strong, slow when weak."""
    return MAX_CLICK_S - (MAX_CLICK_S - MIN_CLICK_S) * cal.fraction(rssi)


def reference_from_samples(samples: list[int]) -> float:
    """Median is robust to the occasional multipath spike while calibrating."""
    if not samples:
        raise ValueError("no samples collected")
    return float(statistics.median(samples))


def default_store() -> Path:
    base = os.environ.get("CCPROX_HOME") or Path.home() / ".ccprox"
    return Path(base) / "calibration.json"


def load_calibrations(path: Path | None = None) -> dict[str, float]:
    path = path or default_store()
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return {}
    return {k.lower(): float(v) for k, v in data.items() if isinstance(v, (int, float))}


def save_calibration(key: str, near: float, path: Path | None = None) -> Path:
    path = path or default_store()
    data = load_calibrations(path)
    data[key.lower()] = near
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(data, indent=2, sort_keys=True), encoding="utf-8")
    return path
