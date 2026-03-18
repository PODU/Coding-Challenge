"""Email delivery + independent delivery retry via the mock."""

from __future__ import annotations

from ai_agent_scheduling.email.client import EmailMessage, MockEmailClient
from ai_agent_scheduling.llm.client import MockLLMClient
from .conftest import make_agent


def test_email_sent_on_success(repo, runner_factory):
    agent = make_agent(name="mail", email_to="ops@example.com")
    repo.create_agent(agent)
    runner, _, email = runner_factory(llm=MockLLMClient(default="the result body"))

    execution = runner.run_once(agent)

    assert execution.success is True
    assert execution.email_delivered is True
    assert len(email.sent) == 1
    sent: EmailMessage = email.sent[0]
    assert sent.to == "ops@example.com"
    assert "mail" in sent.subject
    assert "the result body" in sent.body
    assert "Timestamp:" in sent.body  # agent name, timestamp, formatted response


def test_email_retries_then_succeeds_independently(repo, runner_factory):
    agent = make_agent(name="retrymail", email_to="ops@example.com")
    repo.create_agent(agent)
    email = MockEmailClient(fail_times=2)  # first two attempts fail, third succeeds
    runner, _, _ = runner_factory(email=email)

    execution = runner.run_once(agent)

    assert execution.success is True  # LLM success unaffected
    assert execution.email_delivered is True
    assert email.attempts == 3
    assert len(email.sent) == 1
    # backoff happened between the failed email attempts
    assert runner.sleep_calls == [1.0, 2.0]


def test_email_failure_does_not_fail_the_execution(repo, runner_factory):
    agent = make_agent(name="badmail", email_to="ops@example.com")
    repo.create_agent(agent)
    email = MockEmailClient(always_fail=True)
    runner, _, _ = runner_factory(email=email)

    execution = runner.run_once(agent)

    # LLM execution is still a success; only delivery failed.
    assert execution.success is True
    assert execution.email_delivered is False
    assert execution.email_error is not None
    # persisted with email failure recorded but execution success intact
    persisted = repo.recent_executions("badmail")[0]
    assert persisted.success is True
    assert persisted.email_delivered is False
    assert persisted.email_error is not None
