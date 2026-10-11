"""Geiger-counter audio feedback for hunt mode.

Each counted reading of the target updates the clicker; a background task then
clicks at a tempo set by the smoothed RSSI (faster = closer). If no reading has
arrived within the lost timeout, the clicker goes silent on its own.
"""

from __future__ import annotations

import asyncio
import sys
import time

from .proximity import Calibration, click_interval


def default_sink(strength: float) -> None:
    """Emit one short click. Pitch rises with strength (0..1) where we can."""
    if sys.platform == "win32":
        import winsound

        winsound.Beep(int(600 + 1400 * strength), 25)
    else:
        sys.stdout.write("\a")
        sys.stdout.flush()


class Clicker:
    def __init__(self, cal: Calibration, timeout_s: float, sink=default_sink,
                 clock=time.monotonic) -> None:
        self.cal = cal
        self.timeout_s = timeout_s
        self.sink = sink
        self.clock = clock
        self.smoothed: float | None = None
        self.last_update = float("-inf")
        self.clicks = 0
        self._task: asyncio.Task | None = None

    def update(self, smoothed: float) -> None:
        self.smoothed = smoothed
        self.last_update = self.clock()

    @property
    def active(self) -> bool:
        return self.smoothed is not None and (self.clock() - self.last_update) <= self.timeout_s

    def next_interval(self) -> float | None:
        """Seconds until the next click, or None if silent (target lost)."""
        if not self.active:
            return None
        return click_interval(self.smoothed, self.cal)

    async def _run(self) -> None:
        while True:
            iv = self.next_interval()
            if iv is None:
                await asyncio.sleep(0.1)
                continue
            self.clicks += 1
            strength = self.cal.fraction(self.smoothed)
            await asyncio.to_thread(self.sink, strength)
            await asyncio.sleep(max(0.0, iv - 0.025))

    def start(self) -> None:
        self._task = asyncio.create_task(self._run())

    async def stop(self) -> None:
        if self._task:
            self._task.cancel()
            try:
                await self._task
            except asyncio.CancelledError:
                pass
