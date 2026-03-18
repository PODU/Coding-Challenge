"""Per-agent statistics and health.

An agent is unhealthy once its most recent runs include ``health_failure_threshold``
consecutive failures. A later success clears the streak.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import List

from ..config import Settings
from ..models import AgentStats
from .repository import AgentRepository


@dataclass
class SystemReport:

    total_agents: int
    enabled_agents: int
    disabled_agents: int
    unhealthy_agents: List[str] = field(default_factory=list)
    agents: List[AgentStats] = field(default_factory=list)
    healthy: bool = True


class HealthService:
    def __init__(self, repo: AgentRepository, settings: Settings | None = None):
        self._repo = repo
        self._settings = settings or Settings()

    def agent_stats(self, agent_name: str, *, window: int = 50) -> AgentStats:
        agent = self._repo.get_agent(agent_name)
        rows = self._repo.recent_executions(agent_name, limit=window)  # newest first

        consecutive = 0
        for ex in rows:
            if ex.success:
                break
            consecutive += 1

        success_count = sum(1 for ex in rows if ex.success)
        failure_count = len(rows) - success_count
        last = rows[0] if rows else None
        threshold = self._settings.health_failure_threshold

        return AgentStats(
            agent_name=agent_name,
            enabled=agent.enabled,
            total_runs=len(rows),
            success_count=success_count,
            failure_count=failure_count,
            consecutive_failures=consecutive,
            healthy=consecutive < threshold,
            last_run_at=last.finished_at if last else None,
            last_success=last.success if last else None,
            recent=rows,
        )

    def is_unhealthy(self, agent_name: str) -> bool:
        return not self.agent_stats(agent_name).healthy

    def report(self) -> SystemReport:
        agents = self._repo.list_agents()
        stats = [self.agent_stats(a.name) for a in agents]
        unhealthy = [s.agent_name for s in stats if not s.healthy]
        return SystemReport(
            total_agents=len(agents),
            enabled_agents=sum(1 for a in agents if a.enabled),
            disabled_agents=sum(1 for a in agents if not a.enabled),
            unhealthy_agents=unhealthy,
            agents=stats,
            healthy=not unhealthy,
        )
