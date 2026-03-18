"""Run one agent: LLM call with timeout and backoff retries, then email.

The email step only happens when the LLM call succeeds and has its own retry
loop. ``clock`` and ``sleep`` are injectable so tests run instantly.
"""

from __future__ import annotations

import time
from dataclasses import dataclass
from typing import Callable, List, Optional

from ..config import Settings
from ..email.client import EmailClient, EmailMessage, EmailSendError
from ..llm.client import LLMClient
from ..models import Agent, Execution, utcnow
from .repository import AgentRepository


@dataclass
class _Attempted:
    success: bool
    text: Optional[str]
    error: Optional[str]
    attempts: int


class AgentRunner:
    def __init__(
        self,
        llm: LLMClient,
        email: EmailClient,
        repo: AgentRepository,
        settings: Optional[Settings] = None,
        *,
        clock: Callable[[], object] = utcnow,
        sleep: Callable[[float], None] = time.sleep,
        jitter: Callable[[], float] = lambda: 0.0,
    ):
        self._llm = llm
        self._email = email
        self._repo = repo
        self._settings = settings or Settings()
        self._clock = clock
        self._sleep = sleep
        self._jitter = jitter
        # Every backoff delay, so tests can assert on them.
        self.sleep_calls: List[float] = []

    def run_once(self, agent: Agent) -> Execution:
        """Run the agent, email the result on success, and record the execution."""
        started = self._clock()
        attempted = self._execute_with_retry(agent)
        finished = self._clock()

        execution = Execution(
            agent_id=agent.id,
            agent_name=agent.name,
            started_at=started,  # type: ignore[arg-type]
            finished_at=finished,  # type: ignore[arg-type]
            success=attempted.success,
            response_text=attempted.text,
            error=attempted.error,
            attempts=attempted.attempts,
        )

        if attempted.success and agent.email_to:
            delivered, email_error = self._deliver_with_retry(agent, attempted.text or "", finished)
            execution.email_delivered = delivered
            execution.email_error = email_error

        if self._repo is not None:
            self._repo.save_execution(execution)
        return execution

    def _execute_with_retry(self, agent: Agent) -> _Attempted:
        max_retries = max(1, agent.max_retries)
        last_error: Optional[str] = None

        for attempt in range(max_retries):
            try:
                result = self._llm.complete(
                    system=agent.system_prompt,
                    task=agent.task,
                    timeout_s=agent.timeout_s,
                )
            except BaseException as exc:
                last_error = f"{type(exc).__name__}: {exc}"
            else:
                text = result.text or ""
                if not text.strip():
                    last_error = "empty response from LLM"
                else:
                    return _Attempted(True, text, None, attempt + 1)

            if attempt < max_retries - 1:
                self._backoff(attempt, self._settings.backoff_base_s)

        return _Attempted(False, None, last_error, max_retries)

    def _deliver_with_retry(self, agent: Agent, response_text: str, finished) -> tuple[bool, Optional[str]]:
        message = EmailMessage(
            to=agent.email_to or "",
            subject=f"[AI Agent] {agent.name} — result",
            body=self._format_email_body(agent, response_text, finished),
        )
        max_retries = max(1, self._settings.email_max_retries)
        last_error: Optional[str] = None

        for attempt in range(max_retries):
            try:
                result = self._email.send(message)
            except EmailSendError as exc:
                last_error = str(exc)
            except BaseException as exc:  # pragma: no cover - defensive
                last_error = f"{type(exc).__name__}: {exc}"
            else:
                if result.delivered:
                    return True, None
                last_error = result.error or "delivery reported not delivered"

            if attempt < max_retries - 1:
                self._backoff(attempt, self._settings.email_backoff_base_s)

        return False, last_error

    def _backoff(self, attempt: int, base: float) -> None:
        delay = min(base * (2 ** attempt) + self._jitter(), self._settings.max_backoff_s)
        self.sleep_calls.append(delay)
        self._sleep(delay)

    @staticmethod
    def _format_email_body(agent: Agent, response_text: str, finished) -> str:
        ts = finished.isoformat() if hasattr(finished, "isoformat") else str(finished)
        return (
            f"Agent: {agent.name}\n"
            f"Task: {agent.task}\n"
            f"Timestamp: {ts}\n"
            f"Status: success\n"
            f"\n"
            f"{response_text}\n"
        )
