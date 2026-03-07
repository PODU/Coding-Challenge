"""Facts about a user that persist across sessions.

Simple regexes pick out things like "prefers Go" or "works on the billing
service" from questions; the stored facts are added to later prompts.
"""

from __future__ import annotations

import re
from typing import Protocol, runtime_checkable

# (pattern, prefix for the stored fact)
_PATTERNS = [
    (re.compile(r"\bI\s+prefer\s+(.+?)(?:[.\n]|$)", re.IGNORECASE), "prefers"),
    (re.compile(r"\bI\s+(?:work|am working)\s+on\s+(.+?)(?:[.\n]|$)", re.IGNORECASE), "works on"),
    (re.compile(r"\bI\s+use\s+(.+?)(?:[.\n]|$)", re.IGNORECASE), "uses"),
    (re.compile(r"\bI'?m\s+a\s+(.+?)(?:[.\n]|$)", re.IGNORECASE), "is a"),
    (
        re.compile(r"\bmy\s+(?:team|project|service)\s+(?:is\s+)?(.+?)(?:[.\n]|$)", re.IGNORECASE),
        "team/project:",
    ),
]


def extract_facts(text: str) -> list[str]:
    facts: list[str] = []
    for pat, prefix in _PATTERNS:
        for match in pat.finditer(text):
            phrase = match.group(1).strip().rstrip(".")
            if phrase:
                facts.append(f"{prefix} {phrase}")
    return facts


@runtime_checkable
class MemoryStore(Protocol):
    def remember(self, user_id: str, text: str) -> list[str]: ...

    def recall(self, user_id: str) -> list[str]: ...

    def clear(self, user_id: str | None = None) -> None: ...


class InMemoryLongTermMemory:
    """Keeps facts in a dict, without duplicates."""

    def __init__(self) -> None:
        self._store: dict[str, list[str]] = {}

    def remember(self, user_id: str, text: str) -> list[str]:
        facts = extract_facts(text)
        if not facts:
            return []
        existing = self._store.setdefault(user_id, [])
        added: list[str] = []
        for fact in facts:
            if fact not in existing:
                existing.append(fact)
                added.append(fact)
        return added

    def recall(self, user_id: str) -> list[str]:
        return list(self._store.get(user_id, []))

    def clear(self, user_id: str | None = None) -> None:
        if user_id is None:
            self._store.clear()
        else:
            self._store.pop(user_id, None)


class RedisLongTermMemory:  # pragma: no cover - requires Redis
    """One Redis set per user."""

    def __init__(
        self,
        *,
        url: str = "redis://localhost:6379",
        prefix: str = "rtfm:ltm:",
        client=None,
    ) -> None:
        self._prefix = prefix
        if client is not None:
            self._r = client
        else:
            try:
                import redis  # type: ignore
            except Exception as exc:
                raise RuntimeError(
                    "RedisLongTermMemory requires the optional 'redis' dependency."
                ) from exc
            self._r = redis.Redis.from_url(url)

    def _key(self, user_id: str) -> str:
        return f"{self._prefix}{user_id}"

    def remember(self, user_id: str, text: str) -> list[str]:
        return [f for f in extract_facts(text) if self._r.sadd(self._key(user_id), f)]

    def recall(self, user_id: str) -> list[str]:
        return sorted(m.decode("utf-8") for m in self._r.smembers(self._key(user_id)))

    def clear(self, user_id: str | None = None) -> None:
        if user_id is None:
            keys = self._r.keys(f"{self._prefix}*")
            if keys:
                self._r.delete(*keys)
        else:
            self._r.delete(self._key(user_id))
