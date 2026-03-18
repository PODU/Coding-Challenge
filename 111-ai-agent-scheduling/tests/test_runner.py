"""Agent execution with the mock LLM produces a structured Execution."""

from __future__ import annotations

from datetime import datetime, timezone

from ai_agent_scheduling.llm.client import LLMResult, MockLLMClient
from .conftest import make_agent


def test_run_once_produces_structured_execution(repo, runner_factory):
    agent = make_agent(name="news")
    repo.create_agent(agent)
    llm = MockLLMClient(replies=[LLMResult(text="Today: model X shipped.", model="claude-opus-4-8")])
    runner, _, _ = runner_factory(llm=llm)

    execution = runner.run_once(agent)

    assert execution.agent_name == "news"
    assert execution.success is True
    assert execution.response_text == "Today: model X shipped."
    assert execution.attempts == 1
    assert isinstance(execution.started_at, datetime)
    assert execution.finished_at >= execution.started_at
    # persisted to history
    history = repo.recent_executions("news")
    assert len(history) == 1
    assert history[0].response_text == "Today: model X shipped."


def test_system_prompt_and_task_are_passed_to_llm(repo, runner_factory):
    agent = make_agent(name="p", system_prompt="Be terse.", task="Say hi.")
    repo.create_agent(agent)
    llm = MockLLMClient(default="hi")
    runner, _, _ = runner_factory(llm=llm)
    runner.run_once(agent)
    assert llm.calls[0]["system"] == "Be terse."
    assert llm.calls[0]["task"] == "Say hi."
    assert llm.calls[0]["timeout_s"] == agent.timeout_s


def test_empty_llm_response_counts_as_failure(repo, runner_factory):
    agent = make_agent(name="empty", max_retries=1)
    repo.create_agent(agent)
    llm = MockLLMClient(default="   ")  # whitespace only
    runner, _, _ = runner_factory(llm=llm)
    execution = runner.run_once(agent)
    assert execution.success is False
    assert "empty" in (execution.error or "").lower()


def test_no_email_sent_when_agent_has_no_recipient(repo, runner_factory):
    agent = make_agent(name="noemail", email_to=None)
    repo.create_agent(agent)
    runner, _, email = runner_factory()
    execution = runner.run_once(agent)
    assert execution.success is True
    assert email.sent == []
    assert execution.email_delivered is False
