"""Cron scheduler.

Each tick submits every agent whose next fire time has passed to a thread pool,
so a slow agent does not hold up the others. The clock is injectable so ``tick``
is deterministic in tests.
"""

from __future__ import annotations

import threading
import time
from concurrent.futures import Future, ThreadPoolExecutor
from typing import Callable, Dict, List, Optional

from ..config import Settings
from ..cron.expression import CronExpression
from ..models import Agent, Execution, utcnow
from .repository import AgentRepository
from .runner import AgentRunner


class Scheduler:
    def __init__(
        self,
        repo: AgentRepository,
        runner: AgentRunner,
        settings: Optional[Settings] = None,
        *,
        clock: Callable[[], object] = utcnow,
        executor: Optional[ThreadPoolExecutor] = None,
    ):
        self._repo = repo
        self._runner = runner
        self._settings = settings or Settings()
        self._clock = clock
        self._owns_executor = executor is None
        self._executor = executor or ThreadPoolExecutor(
            max_workers=self._settings.max_parallel_agents,
            thread_name_prefix="agent-runner",
        )
        self._next_fire: Dict[str, object] = {}
        self._lock = threading.Lock()
        self._stop = threading.Event()

    def _schedule_next(self, agent: Agent, after, *, inclusive: bool = False) -> None:
        cron = CronExpression.parse(agent.cron)
        self._next_fire[agent.name] = cron.next_fire(after, inclusive=inclusive)

    def refresh(self) -> None:
        """Sync next-fire times with the set of enabled agents."""
        now = self._clock()
        with self._lock:
            agents = self._repo.list_agents(enabled_only=True)
            enabled = {a.name for a in agents}
            for name in list(self._next_fire):
                if name not in enabled:
                    del self._next_fire[name]
            # Seed inclusively so an agent whose cron matches the current minute
            # fires on this tick instead of a full cycle later.
            for agent in agents:
                if agent.name not in self._next_fire:
                    self._schedule_next(agent, now, inclusive=True)

    def next_fire_time(self, agent_name: str):
        return self._next_fire.get(agent_name)

    def tick(self) -> List[Future]:
        """Submit every agent whose fire time has passed; returns their futures."""
        now = self._clock()
        self.refresh()
        due: List[Agent] = []
        with self._lock:
            for agent in self._repo.list_agents(enabled_only=True):
                fire = self._next_fire.get(agent.name)
                if fire is not None and fire <= now:
                    due.append(agent)
                    # Schedule from the fire time, not now, to stay on the cron grid.
                    self._schedule_next(agent, fire)

        return [self._executor.submit(self._run_agent, agent) for agent in due]

    def _run_agent(self, agent: Agent) -> Execution:
        return self._runner.run_once(agent)

    def run_forever(self, tick_interval: Optional[float] = None) -> None:  # pragma: no cover
        """Tick until ``stop()`` is called, then wait for running agents."""
        interval = tick_interval if tick_interval is not None else self._settings.scheduler_tick_s
        self.refresh()
        try:
            while not self._stop.is_set():
                self.tick()
                self._stop.wait(interval)
        finally:
            self.shutdown()

    def stop(self) -> None:
        self._stop.set()

    def shutdown(self, wait: bool = True) -> None:
        if self._owns_executor:
            self._executor.shutdown(wait=wait)
