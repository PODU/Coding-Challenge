"""Checks that decide when to escalate. ``bot.py`` decides the order and
keeps the repeat-limit counter."""

from __future__ import annotations

import re

from .config import Config
from .text import tokenize

_HUMAN_PATTERNS = [
    re.compile(p, re.IGNORECASE)
    for p in (
        r"\b(speak|talk|chat|connect|transfer|escalate)\b[\w\s,]{0,30}\b"
        r"(human|person|agent|representative|rep|someone|operator|support team)\b",
        r"\b(human|live)\s+(agent|person|support|rep|representative)\b",
        r"\breal\s+(person|human)\b",
        r"\b(get|talk\s+to|speak\s+to)\s+a\s+human\b",
    )
]


class EscalationPolicy:
    def __init__(self, config: Config) -> None:
        self.config = config

    def is_human_request(self, text: str) -> bool:
        return any(p.search(text) for p in _HUMAN_PATTERNS)

    def is_off_topic(self, text: str, domain_terms: set[str]) -> bool:
        """True when the query shares no words with the knowledge base."""
        tokens = set(tokenize(text))
        if not tokens or not domain_terms:
            return False
        return tokens.isdisjoint(domain_terms)

    def is_abstention(self, answer_text: str) -> bool:
        """True when the model replied with the "can't answer" sentinel."""
        return self.config.abstain_sentinel.lower() in answer_text.lower()
