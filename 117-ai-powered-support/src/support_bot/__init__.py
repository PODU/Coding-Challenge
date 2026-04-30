"""A command-line support bot that answers from a small knowledge base.

Retrieval, the LLM, and metrics all sit behind interfaces, so the tests run
offline.
"""

from __future__ import annotations

from .bot import SupportBot
from .config import Config
from .factory import build_bot
from .models import Answer, Escalation, EscalationReason

__all__ = [
    "SupportBot",
    "Config",
    "build_bot",
    "Answer",
    "Escalation",
    "EscalationReason",
]

__version__ = "0.1.0"
