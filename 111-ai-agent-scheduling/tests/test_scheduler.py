"""Scheduler dispatch with an injected clock: parallel, non-blocking."""

from __future__ import annotations

import threading
import time
from datetime import datetime, timezone

from ai_agent_scheduling.config import Settings
from ai_agent_scheduling.core import AgentRepository, AgentRunner, Scheduler
from ai_agent_scheduling.email.client import MockEmailClient
from ai_agent_scheduling.llm.client import MockLLMClient
from .conftest import make_agent


def _wait(futures):
    return [f.result(timeout=5) for f in futures]


def test_two_agents_due_at_same_instant_both_run(repo, settings):
    a = make_agent(name="a", cron="0 9 * * *")
    b = make_agent(name="b", cron="0 9 * * *")
    repo.create_agent(a)
    repo.create_agent(b)

    clock = {"now": datetime(2025, 6, 2, 8, 0, tzinfo=timezone.utc)}
    runner = AgentRunner(MockLLMClient(default="ok"), MockEmailClient(), repo, settings, sleep=lambda d: None)
    sched = Scheduler(repo, runner, settings, clock=lambda: clock["now"])

    # Before 09:00 nothing is due.
    assert _wait(sched.tick()) == []

    # Advance past the fire time -> both agents dispatched.
    clock["now"] = datetime(2025, 6, 2, 9, 0, tzinfo=timezone.utc)
    executions = _wait(sched.tick())
    names = sorted(e.agent_name for e in executions)
    assert names == ["a", "b"]
    assert all(e.success for e in executions)
    sched.shutdown()


def test_disabled_agent_is_skipped(repo, settings):
    repo.create_agent(make_agent(name="on", cron="0 9 * * *", enabled=True))
    repo.create_agent(make_agent(name="off", cron="0 9 * * *", enabled=False))
    clock = {"now": datetime(2025, 6, 2, 9, 0, tzinfo=timezone.utc)}
    runner = AgentRunner(MockLLMClient(default="ok"), MockEmailClient(), repo, settings, sleep=lambda d: None)
    sched = Scheduler(repo, runner, settings, clock=lambda: clock["now"])
    executions = _wait(sched.tick())
    assert [e.agent_name for e in executions] == ["on"]
    sched.shutdown()


def test_not_refired_until_next_cron_slot(repo, settings):
    repo.create_agent(make_agent(name="hourly", cron="0 * * * *"))
    clock = {"now": datetime(2025, 6, 2, 9, 0, tzinfo=timezone.utc)}
    runner = AgentRunner(MockLLMClient(default="ok"), MockEmailClient(), repo, settings, sleep=lambda d: None)
    sched = Scheduler(repo, runner, settings, clock=lambda: clock["now"])

    assert len(_wait(sched.tick())) == 1
    # Same minute, tick again -> not due again.
    assert _wait(sched.tick()) == []
    # Advance an hour -> due again.
    clock["now"] = datetime(2025, 6, 2, 10, 0, tzinfo=timezone.utc)
    assert len(_wait(sched.tick())) == 1
    sched.shutdown()


def test_slow_agent_does_not_block_others(settings):
    """A blocking agent runs in parallel; a fast agent finishes independently."""
    repo = AgentRepository(":memory:")
    repo.create_agent(make_agent(name="slow", cron="0 9 * * *", task="slow-marker task"))
    repo.create_agent(make_agent(name="fast", cron="0 9 * * *", task="fast task"))

    gate = threading.Event()

    class RoutingLLM:
        """Blocks the 'slow-marker' agent until the gate is released."""

        def complete(self, *, system, task, timeout_s):
            from ai_agent_scheduling.llm.client import LLMResult

            if "slow-marker" in task:
                gate.wait(5)
                return LLMResult(text="slow done", model="m")
            return LLMResult(text="fast done", model="m")

    pool_settings = Settings(max_parallel_agents=4)
    runner = AgentRunner(RoutingLLM(), MockEmailClient(), repo, pool_settings, sleep=lambda d: None)
    sched = Scheduler(
        repo, runner, pool_settings,
        clock=lambda: datetime(2025, 6, 2, 9, 0, tzinfo=timezone.utc),
    )

    futures = sched.tick()
    # The fast agent should complete even while the slow one is still blocked.
    fast_future = None
    deadline = time.time() + 5
    while time.time() < deadline:
        done = [f for f in futures if f.done()]
        if any(f.result().agent_name == "fast" for f in done):
            fast_future = next(f for f in done if f.result().agent_name == "fast")
            break
        time.sleep(0.01)
    assert fast_future is not None, "fast agent was blocked by the slow one"

    gate.set()  # release the slow agent
    _wait(futures)
    sched.shutdown()
    repo.close()
