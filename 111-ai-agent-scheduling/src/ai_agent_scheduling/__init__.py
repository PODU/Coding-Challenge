"""Run LLM tasks on a cron schedule and email the results.

The LLM and email providers sit behind ``LLMClient`` / ``EmailClient`` so the
rest of the package can be tested offline.
"""

from .config import LLM_MODEL, Settings
from .cron import CronExpression, CronParser, CronParseResult
from .email import EmailMessage, MockEmailClient
from .llm import AnthropicLLMClient, LLMResult, MockLLMClient
from .models import Agent, AgentStats, Execution
from .core import (
    AgentRepository,
    AgentRunner,
    HealthService,
    Scheduler,
)

__version__ = "1.0.0"

__all__ = [
    "Settings",
    "LLM_MODEL",
    "Agent",
    "Execution",
    "AgentStats",
    "CronExpression",
    "CronParser",
    "CronParseResult",
    "LLMResult",
    "AnthropicLLMClient",
    "MockLLMClient",
    "EmailMessage",
    "MockEmailClient",
    "AgentRepository",
    "AgentRunner",
    "Scheduler",
    "HealthService",
]
