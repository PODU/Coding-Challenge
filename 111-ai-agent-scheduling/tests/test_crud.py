"""CRUD persistence + execution history against a temp SQLite DB."""

from __future__ import annotations

import pytest

from ai_agent_scheduling.core import (
    AgentNotFoundError,
    AgentRepository,
    DuplicateAgentError,
)
from ai_agent_scheduling.models import Agent
from .conftest import make_agent


def test_create_and_get_roundtrip(tmp_path):
    db = tmp_path / "agents.db"
    with AgentRepository(str(db)) as repo:
        repo.create_agent(make_agent(name="a", email_to="x@y.z"))

    # reopen to prove it persisted to disk
    with AgentRepository(str(db)) as repo:
        got = repo.get_agent("a")
        assert got.id is not None
        assert got.email_to == "x@y.z"
        assert got.cron == "0 9 * * 1-5"
        assert got.enabled is True
        assert got.created_at is not None


def test_duplicate_name_rejected(repo):
    repo.create_agent(make_agent(name="dup"))
    with pytest.raises(DuplicateAgentError):
        repo.create_agent(make_agent(name="dup"))


def test_list_and_enabled_filter(repo):
    repo.create_agent(make_agent(name="one", enabled=True))
    repo.create_agent(make_agent(name="two", enabled=False))
    assert [a.name for a in repo.list_agents()] == ["one", "two"]
    assert [a.name for a in repo.list_agents(enabled_only=True)] == ["one"]


def test_update_agent(repo):
    repo.create_agent(make_agent(name="up", task="old"))
    agent = repo.get_agent("up")
    agent.task = "new task"
    agent.cron = "*/10 * * * *"
    repo.update_agent(agent)
    got = repo.get_agent("up")
    assert got.task == "new task"
    assert got.cron == "*/10 * * * *"


def test_set_enabled(repo):
    repo.create_agent(make_agent(name="toggle"))
    repo.set_enabled("toggle", False)
    assert repo.get_agent("toggle").enabled is False
    repo.set_enabled("toggle", True)
    assert repo.get_agent("toggle").enabled is True


def test_delete_agent_and_cascade_history(repo):
    agent = repo.create_agent(make_agent(name="del"))
    from ai_agent_scheduling.models import Execution, utcnow

    repo.save_execution(
        Execution(
            agent_id=agent.id,
            agent_name="del",
            started_at=utcnow(),
            finished_at=utcnow(),
            success=True,
            attempts=1,
        )
    )
    assert len(repo.recent_executions("del")) == 1
    repo.delete_agent("del")
    with pytest.raises(AgentNotFoundError):
        repo.get_agent("del")


def test_delete_missing_raises(repo):
    with pytest.raises(AgentNotFoundError):
        repo.delete_agent("ghost")


def test_end_to_end_create_run_persist_status(tmp_path):
    """create -> run_once -> persisted history -> status, all offline."""
    from ai_agent_scheduling.config import Settings
    from ai_agent_scheduling.core import AgentRunner, HealthService
    from ai_agent_scheduling.email.client import MockEmailClient
    from ai_agent_scheduling.llm.client import MockLLMClient

    settings = Settings(db_path=str(tmp_path / "e2e.db"))
    with AgentRepository(settings.db_path) as repo:
        agent = repo.create_agent(make_agent(name="e2e", email_to="ops@x.z"))
        runner = AgentRunner(
            MockLLMClient(default="report body"),
            MockEmailClient(),
            repo,
            settings,
            sleep=lambda d: None,
        )
        execution = runner.run_once(agent)
        assert execution.success is True

        history = repo.recent_executions("e2e")
        assert len(history) == 1
        assert history[0].email_delivered is True

        report = HealthService(repo, settings).report()
        assert report.total_agents == 1
        assert report.healthy is True
