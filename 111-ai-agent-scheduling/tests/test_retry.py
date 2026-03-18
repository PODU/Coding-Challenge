"""Timeout & exponential-backoff retry on mock failures."""

from __future__ import annotations

from ai_agent_scheduling.llm.client import LLMResult, LLMTimeoutError, MockLLMClient
from .conftest import make_agent


def test_retries_then_succeeds_with_exponential_backoff(repo, runner_factory):
    agent = make_agent(name="flaky", max_retries=3)
    repo.create_agent(agent)
    # First two attempts raise (timeout, then generic error), third succeeds.
    llm = MockLLMClient(
        replies=[LLMTimeoutError("boom"), RuntimeError("502"), LLMResult(text="ok now", model="m")]
    )
    runner, _, _ = runner_factory(llm=llm)

    execution = runner.run_once(agent)

    assert execution.success is True
    assert execution.attempts == 3
    assert execution.response_text == "ok now"
    # Two backoff sleeps recorded: base*2**0 and base*2**1 == 1.0, 2.0
    assert runner.sleep_calls == [1.0, 2.0]


def test_all_attempts_fail_records_failed_execution(repo, runner_factory):
    agent = make_agent(name="dead", max_retries=3)
    repo.create_agent(agent)
    llm = MockLLMClient(replies=[LLMTimeoutError("t1"), LLMTimeoutError("t2"), LLMTimeoutError("t3")])
    runner, _, email = runner_factory(llm=llm)

    execution = runner.run_once(agent)

    assert execution.success is False
    assert execution.attempts == 3
    assert "LLMTimeoutError" in (execution.error or "")
    assert email.sent == []  # no email on failure
    # backoff happened before attempts 2 and 3 only (not after the final attempt)
    assert runner.sleep_calls == [1.0, 2.0]


def test_failed_run_does_not_affect_other_agents(repo, runner_factory):
    bad = make_agent(name="bad", max_retries=1)
    good = make_agent(name="good", max_retries=1)
    repo.create_agent(bad)
    repo.create_agent(good)

    bad_llm = MockLLMClient(replies=[LLMTimeoutError("x")])
    bad_runner, _, _ = runner_factory(llm=bad_llm)
    bad_exec = bad_runner.run_once(bad)

    good_llm = MockLLMClient(default="fine")
    good_runner, _, _ = runner_factory(llm=good_llm)
    good_exec = good_runner.run_once(good)

    assert bad_exec.success is False
    assert good_exec.success is True
    # Each agent's history is independent.
    assert len(repo.recent_executions("bad")) == 1
    assert len(repo.recent_executions("good")) == 1


def test_single_attempt_no_backoff(repo, runner_factory):
    agent = make_agent(name="once", max_retries=1)
    repo.create_agent(agent)
    llm = MockLLMClient(default="quick")
    runner, _, _ = runner_factory(llm=llm)
    execution = runner.run_once(agent)
    assert execution.attempts == 1
    assert runner.sleep_calls == []
