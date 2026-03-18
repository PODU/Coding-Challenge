"""``Agent`` configuration and ``Execution`` history rows."""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Optional

from .config import Settings


def utcnow() -> datetime:
    return datetime.now(timezone.utc)


@dataclass
class Agent:
    """A named LLM task with its schedule and delivery settings."""

    name: str
    task: str
    system_prompt: Optional[str] = None
    cron: str = "0 * * * *"
    timeout_s: float = 60.0
    max_retries: int = 3
    email_to: Optional[str] = None
    enabled: bool = True
    id: Optional[int] = None
    created_at: Optional[datetime] = None
    updated_at: Optional[datetime] = None

    @classmethod
    def new(
        cls,
        name: str,
        task: str,
        cron: str,
        *,
        system_prompt: Optional[str] = None,
        email_to: Optional[str] = None,
        enabled: bool = True,
        settings: Optional[Settings] = None,
        timeout_s: Optional[float] = None,
        max_retries: Optional[int] = None,
    ) -> "Agent":
        """Create an agent, defaulting timeout/retries from ``settings``."""
        settings = settings or Settings()
        return cls(
            name=name,
            task=task,
            system_prompt=system_prompt,
            cron=cron,
            timeout_s=timeout_s if timeout_s is not None else settings.default_timeout_s,
            max_retries=max_retries if max_retries is not None else settings.default_max_retries,
            email_to=email_to,
            enabled=enabled,
        )


@dataclass
class Execution:
    """One recorded run of an agent."""

    agent_name: str
    started_at: datetime
    finished_at: datetime
    success: bool
    response_text: Optional[str] = None
    error: Optional[str] = None
    attempts: int = 0
    email_delivered: bool = False
    email_error: Optional[str] = None
    id: Optional[int] = None
    agent_id: Optional[int] = None

    def as_dict(self) -> dict:
        return {
            "id": self.id,
            "agent_id": self.agent_id,
            "agent_name": self.agent_name,
            "started_at": self.started_at.isoformat(),
            "finished_at": self.finished_at.isoformat(),
            "success": self.success,
            "response_text": self.response_text,
            "error": self.error,
            "attempts": self.attempts,
            "email_delivered": self.email_delivered,
            "email_error": self.email_error,
        }


@dataclass
class AgentStats:
    """Run statistics and health for one agent."""

    agent_name: str
    enabled: bool
    total_runs: int
    success_count: int
    failure_count: int
    consecutive_failures: int
    healthy: bool
    last_run_at: Optional[datetime] = None
    last_success: Optional[bool] = None
    recent: list = field(default_factory=list)

    @property
    def success_rate(self) -> float:
        return (self.success_count / self.total_runs) if self.total_runs else 0.0
