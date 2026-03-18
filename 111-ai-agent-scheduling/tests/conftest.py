"""Shared fixtures. Everything here is offline — no API key, no network."""

from __future__ import annotations

from datetime import datetime, timezone

import pytest

from ai_agent_scheduling.config import Settings
from ai_agent_scheduling.core import AgentRepository, AgentRunner
from ai_agent_scheduling.email.client import MockEmailClient
from ai_agent_scheduling.llm.client import MockLLMClient
from ai_agent_scheduling.models import Agent


@pytest.fixture
def settings() -> Settings:
    # Tiny backoff base so any (mocked) sleep math is easy to assert on.
    return Settings(backoff_base_s=1.0, email_backoff_base_s=1.0, max_backoff_s=1000.0)


@pytest.fixture
def repo() -> AgentRepository:
    r = AgentRepository(":memory:")
    yield r
    r.close()


@pytest.fixture
def recorded_sleep():
    calls: list[float] = []

    def _sleep(d: float) -> None:
        calls.append(d)

    _sleep.calls = calls  # type: ignore[attr-defined]
    return _sleep


@pytest.fixture
def fixed_clock():
    """A monotonic, deterministic clock returning successive UTC instants."""
    base = datetime(2025, 6, 2, 9, 0, 0, tzinfo=timezone.utc)  # a Monday
    state = {"t": base}

    def _clock():
        now = state["t"]
        return now

    _clock.set = lambda dt: state.__setitem__("t", dt)  # type: ignore[attr-defined]
    return _clock


def make_agent(name="demo", **kw) -> Agent:
    defaults = dict(
        task="Summarize today's top AI news.",
        cron="0 9 * * 1-5",
        system_prompt="You are a concise assistant.",
        email_to=None,
        max_retries=3,
        timeout_s=60.0,
    )
    defaults.update(kw)
    return Agent(name=name, **defaults)


@pytest.fixture
def runner_factory(repo, settings, recorded_sleep):
    """Build an AgentRunner with injected mock clients + recorded sleep."""

    def _make(llm=None, email=None, *, clock=None):
        llm = llm or MockLLMClient(default="A concise summary.")
        email = email or MockEmailClient()
        kwargs = {"sleep": recorded_sleep}
        if clock is not None:
            kwargs["clock"] = clock
        runner = AgentRunner(llm, email, repo, settings, **kwargs)
        return runner, llm, email

    return _make
