"""Health flagging after three consecutive failures."""

from __future__ import annotations

from datetime import datetime, timedelta, timezone

from ai_agent_scheduling.core import HealthService
from ai_agent_scheduling.models import Execution
from .conftest import make_agent


def _seed_execution(repo, agent, success, when):
    repo.save_execution(
        Execution(
            agent_id=agent.id,
            agent_name=agent.name,
            started_at=when,
            finished_at=when,
            success=success,
            response_text="ok" if success else None,
            error=None if success else "boom",
            attempts=1,
        )
    )


def test_three_consecutive_failures_flag_unhealthy(repo, settings):
    agent = make_agent(name="watch")
    repo.create_agent(agent)
    base = datetime(2025, 6, 2, 9, 0, tzinfo=timezone.utc)

    health = HealthService(repo, settings)

    # one success, then two failures -> still healthy
    _seed_execution(repo, agent, True, base)
    _seed_execution(repo, agent, False, base + timedelta(minutes=1))
    _seed_execution(repo, agent, False, base + timedelta(minutes=2))
    stats = health.agent_stats("watch")
    assert stats.consecutive_failures == 2
    assert stats.healthy is True

    # third consecutive failure -> unhealthy
    _seed_execution(repo, agent, False, base + timedelta(minutes=3))
    stats = health.agent_stats("watch")
    assert stats.consecutive_failures == 3
    assert stats.healthy is False
    assert health.is_unhealthy("watch") is True


def test_later_success_clears_the_streak(repo, settings):
    agent = make_agent(name="recover")
    repo.create_agent(agent)
    base = datetime(2025, 6, 2, 9, 0, tzinfo=timezone.utc)
    health = HealthService(repo, settings)

    for i in range(3):
        _seed_execution(repo, agent, False, base + timedelta(minutes=i))
    assert health.agent_stats("recover").healthy is False

    # a fresh success resets consecutive failures to 0
    _seed_execution(repo, agent, True, base + timedelta(minutes=3))
    stats = health.agent_stats("recover")
    assert stats.consecutive_failures == 0
    assert stats.healthy is True
    assert stats.last_success is True


def test_system_report_lists_unhealthy_agents(repo, settings):
    ok = make_agent(name="ok")
    sick = make_agent(name="sick")
    repo.create_agent(ok)
    repo.create_agent(sick)
    base = datetime(2025, 6, 2, 9, 0, tzinfo=timezone.utc)
    _seed_execution(repo, ok, True, base)
    for i in range(3):
        _seed_execution(repo, sick, False, base + timedelta(minutes=i))

    report = HealthService(repo, settings).report()
    assert report.total_agents == 2
    assert report.healthy is False
    assert report.unhealthy_agents == ["sick"]


def test_success_rate_computation(repo, settings):
    agent = make_agent(name="rate")
    repo.create_agent(agent)
    base = datetime(2025, 6, 2, 9, 0, tzinfo=timezone.utc)
    _seed_execution(repo, agent, True, base)
    _seed_execution(repo, agent, True, base + timedelta(minutes=1))
    _seed_execution(repo, agent, False, base + timedelta(minutes=2))
    stats = HealthService(repo, settings).agent_stats("rate")
    assert stats.total_runs == 3
    assert stats.success_count == 2
    assert abs(stats.success_rate - 2 / 3) < 1e-9
