"""Rules about which challenges may be recommended (difficulty ceiling,
blocked topics). Each decision is recorded as a span event."""

from __future__ import annotations

from dataclasses import dataclass, field

from .telemetry import Span


@dataclass(frozen=True)
class ChallengeInfo:
    name: str
    difficulty: str  # "beginner" | "intermediate" | "advanced"
    topics: tuple[str, ...] = ()


@dataclass
class ChallengePolicy:
    max_difficulty: str = "advanced"
    blocked_topics: frozenset[str] = field(default_factory=frozenset)

    _ORDER = ("beginner", "intermediate", "advanced")

    def _allowed_difficulty(self, difficulty: str) -> bool:
        order = self._ORDER
        if difficulty not in order or self.max_difficulty not in order:
            return True
        return order.index(difficulty) <= order.index(self.max_difficulty)

    def check(self, challenge: ChallengeInfo) -> tuple[bool, str]:
        if not self._allowed_difficulty(challenge.difficulty):
            return False, (
                f"difficulty {challenge.difficulty!r} above policy ceiling "
                f"{self.max_difficulty!r}"
            )
        blocked = self.blocked_topics.intersection(challenge.topics)
        if blocked:
            return False, f"blocked topics: {', '.join(sorted(blocked))}"
        return True, "allowed"

    def filter(
        self, challenges: list[ChallengeInfo], *, span: Span | None = None
    ) -> list[ChallengeInfo]:
        allowed: list[ChallengeInfo] = []
        for challenge in challenges:
            ok, reason = self.check(challenge)
            if span is not None:
                span.add_event(
                    "policy.enforcement",
                    {"challenge": challenge.name, "allowed": ok, "reason": reason},
                )
            if ok:
                allowed.append(challenge)
        return allowed
