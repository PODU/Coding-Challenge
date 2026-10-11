import asyncio
import io
import time

import pytest

from ccprox.tracker import Reading


class ScriptedSource:
    """Emits (delay_s, address, name, rssi) events in real time, then idles."""

    def __init__(self, queue, clock, events):
        self.queue, self.clock, self.events = queue, clock, events
        self.task = None

    async def _run(self):
        for delay, addr, name, rssi in self.events:
            await asyncio.sleep(delay)
            self.queue.put_nowait(Reading(addr, name, rssi, self.clock()))

    async def start(self):
        self.task = asyncio.create_task(self._run())

    async def stop(self):
        self.task.cancel()
        try:
            await self.task
        except asyncio.CancelledError:
            pass


@pytest.fixture
def run_cli(tmp_path, monkeypatch):
    """run_cli(argv, events=None, sink=None, source_factory=None) -> (code, stdout, stderr)"""
    monkeypatch.setenv("CCPROX_HOME", str(tmp_path))

    def run(argv, events=None, sink=None, source_factory=None):
        from ccprox.cli import main

        out, err = io.StringIO(), io.StringIO()
        if source_factory is None and events is not None:
            source_factory = lambda q, clock: ScriptedSource(q, clock, events)
        code = main(argv, out=out, err=err, source_factory=source_factory,
                    clock=time.monotonic, sink=sink)
        return code, out.getvalue(), err.getvalue()

    return run
